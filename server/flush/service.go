package flush

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
	"tiny-file-watcher/server/database"
	tfwssh "tiny-file-watcher/server/ssh"

	pb "tiny-file-watcher/gen/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SFTPDialer opens an SFTP-like client for a given machine.
// In production the real SSH implementation is used; tests inject a local-fs fake.
type SFTPDialer interface {
	Dial(cfg tfwssh.MachineConfig) (SFTPClient, error)
}

// SFTPClient is the subset of *sftp.Client operations used during flush.
type SFTPClient interface {
	Stat(path string) (os.FileInfo, error)
	MkdirAll(path string) error
	Open(path string) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	Chmod(path string, mode os.FileMode) error
	Chtimes(path string, atime, mtime time.Time) error
	Close() error
}

// sshDialer is the production SFTPDialer that establishes real SSH+SFTP connections.
type sshDialer struct{}

func (sshDialer) Dial(cfg tfwssh.MachineConfig) (SFTPClient, error) {
	c, err := tfwssh.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &sshSFTPClient{c: c}, nil
}

// sshSFTPClient wraps *tfwssh.Client to satisfy SFTPClient and close both SFTP and SSH on Close.
type sshSFTPClient struct{ c *tfwssh.Client }

func (s *sshSFTPClient) Stat(path string) (os.FileInfo, error) { return s.c.SFTPClient.Stat(path) }
func (s *sshSFTPClient) MkdirAll(path string) error            { return s.c.SFTPClient.MkdirAll(path) }
func (s *sshSFTPClient) Chmod(path string, mode os.FileMode) error {
	return s.c.SFTPClient.Chmod(path, mode)
}
func (s *sshSFTPClient) Chtimes(path string, a, m time.Time) error {
	return s.c.SFTPClient.Chtimes(path, a, m)
}
func (s *sshSFTPClient) Close() error { s.c.Close(); return nil }

func (s *sshSFTPClient) Open(path string) (io.ReadCloser, error) {
	return s.c.SFTPClient.Open(path)
}

func (s *sshSFTPClient) Create(path string) (io.WriteCloser, error) {
	return s.c.SFTPClient.Create(path)
}

// localDialer is an SFTPDialer backed by the local filesystem, used in tests.
type localDialer struct{}

// LocalDialer returns an SFTPDialer that operates on the local filesystem.
// Intended for use in tests.
func LocalDialer() SFTPDialer { return localDialer{} }

func (localDialer) Dial(_ tfwssh.MachineConfig) (SFTPClient, error) {
	return &localSFTPClient{}, nil
}

type localSFTPClient struct{}

func (localSFTPClient) Stat(path string) (os.FileInfo, error)     { return os.Stat(path) }
func (localSFTPClient) MkdirAll(path string) error                { return os.MkdirAll(path, 0o755) }
func (localSFTPClient) Chmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }
func (localSFTPClient) Chtimes(path string, a, m time.Time) error { return os.Chtimes(path, a, m) }
func (localSFTPClient) Close() error                              { return nil }

func (localSFTPClient) Open(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func (localSFTPClient) Create(path string) (io.WriteCloser, error) {
	return os.Create(path)
}

// FlushService implements the FileFlushService gRPC server.
type FlushService struct {
	pb.UnimplementedFileFlushServiceServer
	flushRepository database.FlushRepository
	dialer          SFTPDialer
	logger          *slog.Logger
}

func NewFlushService(flushRepository database.FlushRepository, logger *slog.Logger) *FlushService {
	return NewFlushServiceWithDialer(flushRepository, sshDialer{}, logger)
}

// NewFlushServiceWithDialer creates a FlushService with a custom SFTPDialer (intended for tests).
func NewFlushServiceWithDialer(flushRepository database.FlushRepository, dialer SFTPDialer, logger *slog.Logger) *FlushService {
	return &FlushService{flushRepository: flushRepository, dialer: dialer, logger: logger}
}

func (s *FlushService) StreamFlushWatcher(req *pb.FlushWatcherRequest, stream grpc.ServerStreamingServer[pb.FlushWatcherEvent]) error {
	if req.Name == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}

	sendLog := func(msg string) error {
		return stream.Send(&pb.FlushWatcherEvent{
			Type:    pb.FlushWatcherEvent_LOG,
			Message: msg,
		})
	}

	logCallback := WithLogCallback(func(msg string) {
		_ = sendLog(msg)
	})

	job := NewFlushJob(req.Name, s.logger, s.flushRepository, &s.dialer, logCallback)

	result, err := job.Run()
	if err != nil {
		return status.Errorf(codes.Internal, "flush watcher: %v", err)
	}

	if err := sendLog("flush job completed"); err != nil {
		return status.Errorf(codes.Internal, "stream send: %v", err)
	}

	if err := stream.Send(&pb.FlushWatcherEvent{
		Type:   pb.FlushWatcherEvent_RESULT,
		Result: &pb.FlushWatcherResponse{Success: result},
	}); err != nil {
		return status.Errorf(codes.Internal, "stream send: %v", err)
	}

	return nil
}

func (s *FlushService) ListPendingFiles(_ context.Context, req *pb.ListPendingFilesRequest) (*pb.ListPendingFilesResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	pendings, err := s.flushRepository.ListPendingFlushes(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list pending flushes: %v", err)
	}

	files := make([]*pb.WatchedFile, 0, len(pendings))
	for _, pf := range pendings {
		files = append(files, &pb.WatchedFile{
			Id:        pf.WatchedFileID,
			WatcherId: pf.WatcherName,
			FilePath:  pf.FilePath,
			FileName:  pf.FileName,
			Flushed:   false,
		})
	}

	return &pb.ListPendingFilesResponse{Files: files}, nil
}

func (s *FlushService) FlushWatcher(_ context.Context, req *pb.FlushWatcherRequest) (*pb.FlushWatcherResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	job := NewFlushJob(req.Name, s.logger, s.flushRepository, &s.dialer)

	result, err := job.Run()
	if err != nil {
		return &pb.FlushWatcherResponse{
			Success: false,
		}, status.Errorf(codes.Internal, "flush watcher: %v", err)
	}

	return &pb.FlushWatcherResponse{Success: result}, nil
}

// ensure FlushService satisfies the generated interface at compile time.
var _ pb.FileFlushServiceServer = (*FlushService)(nil)
