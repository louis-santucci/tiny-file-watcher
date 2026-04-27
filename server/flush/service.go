package flush

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"tiny-file-watcher/server/database"
	tfwssh "tiny-file-watcher/server/ssh"

	pb "tiny-file-watcher/gen/grpc"

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

func (s *FlushService) FlushWatcher(_ context.Context, req *pb.FlushWatcherRequest) (*pb.FlushWatcherResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	pendings, err := s.flushRepository.ListPendingFlushes(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list pending flushes: %v", err)
	}

	if len(pendings) == 0 {
		return &pb.FlushWatcherResponse{Success: true}, nil
	}

	// Pool SFTP clients keyed by machine name to avoid one handshake per file.
	clients := make(map[string]SFTPClient)
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	getClient := func(name, ip string, port int32, user, keyPath string) (SFTPClient, error) {
		if c, ok := clients[name]; ok {
			return c, nil
		}
		c, err := s.dialer.Dial(tfwssh.MachineConfig{
			Name:              name,
			IP:                ip,
			SSHPort:           port,
			SSHUser:           user,
			SSHPrivateKeyPath: keyPath,
		})
		if err != nil {
			return nil, err
		}
		clients[name] = c
		return c, nil
	}

	ids := make([]int64, 0, len(pendings))
	for _, pf := range pendings {
		srcClient, err := getClient(
			pf.MachineName, pf.MachineIP, pf.MachineSSHPort,
			pf.MachineSSHUser, pf.MachineSSHPrivateKeyPath,
		)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "connect to source machine %s: %v", pf.MachineName, err)
		}

		// Reuse the same client when source == target machine.
		var dstClient SFTPClient
		if pf.MachineName == pf.TargetMachineName {
			dstClient = srcClient
		} else {
			dstClient, err = getClient(
				pf.TargetMachineName, pf.TargetMachineIP, pf.TargetMachineSSHPort,
				pf.TargetMachineSSHUser, pf.TargetMachineSSHPrivateKeyPath,
			)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "connect to target machine %s: %v", pf.TargetMachineName, err)
			}
		}

		src := filepath.Join(pf.FilePath, pf.FileName)
		dst := filepath.Join(pf.TargetPath, pf.FileName)
		if err := transferFile(srcClient, dstClient, src, dst); err != nil {
			return nil, status.Errorf(codes.Internal, "transfer file %s: %v", src, err)
		}
		ids = append(ids, pf.WatchedFileID)
	}

	if err := s.flushRepository.FlushWatchedFiles(ids); err != nil {
		return nil, status.Errorf(codes.Internal, "mark files flushed: %v", err)
	}

	return &pb.FlushWatcherResponse{Success: true}, nil
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

// transferFile copies a file from src on srcClient to dst on dstClient,
// preserving permissions and modification time.
func transferFile(srcClient, dstClient SFTPClient, src, dst string) error {
	srcInfo, err := srcClient.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if err := dstClient.MkdirAll(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("mkdir destination: %w", err)
	}

	in, err := srcClient.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := dstClient.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	if err := dstClient.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("chmod destination: %w", err)
	}

	modTime := srcInfo.ModTime()
	if err := dstClient.Chtimes(dst, modTime, modTime); err != nil {
		return fmt.Errorf("chtimes destination: %w", err)
	}

	return nil
}

// ensure FlushService satisfies the generated interface at compile time.
var _ pb.FileFlushServiceServer = (*FlushService)(nil)
