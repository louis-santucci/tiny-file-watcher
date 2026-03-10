package flush

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	pb "tiny-file-watcher/gen/grpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FlushService implements the FileFlushService gRPC server.
type FlushService struct {
	pb.UnimplementedFileFlushServiceServer
	flushRepository FlushRepository
}

func NewFlushService(flushRepository FlushRepository) *FlushService {
	return &FlushService{flushRepository: flushRepository}
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

	ids := make([]int64, 0, len(pendings))
	for _, pf := range pendings {
		if err := copyFile(pf.FilePath, filepath.Join(pf.TargetPath, pf.FileName)); err != nil {
			return nil, status.Errorf(codes.Internal, "copy file %s: %v", pf.FilePath, err)
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
			Flushed:   false,
		})
	}

	return &pb.ListPendingFilesResponse{Files: files}, nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}
	return out.Sync()
}

// ensure FlushService satisfies the generated interface at compile time.
var _ pb.FileFlushServiceServer = (*FlushService)(nil)
