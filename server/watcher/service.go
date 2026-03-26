package watcher

import (
	"context"
	"io/fs"
	"log/slog"
	"path/filepath"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
)

// WatcherService implements the FileWatcherService gRPC server.
type WatcherService struct {
	pb.UnimplementedFileWatcherServiceServer
	fileWatcherRepository FileWatcherRepository
	fileRepository        FileRepository
	filterRepository      FilterRepository
	logger                *slog.Logger
}

func NewManagerService(fileWatcherRepository FileWatcherRepository, fileRepository FileRepository, filterRepository FilterRepository, logger *slog.Logger) *WatcherService {
	return &WatcherService{
		fileWatcherRepository: fileWatcherRepository,
		fileRepository:        fileRepository,
		filterRepository:      filterRepository,
		logger:                logger,
	}
}

func (s *WatcherService) CreateWatcher(_ context.Context, req *pb.CreateWatcherRequest) (*pb.Watcher, error) {
	if req.Name == "" || req.SourcePath == "" {
		return nil, status.Error(codes.InvalidArgument, "name and source_path are required")
	}
	w, err := s.fileWatcherRepository.CreateWatcher(req.Name, req.SourcePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create watcher: %v", err)
	}
	return toProto(w), nil
}

func (s *WatcherService) GetWatcherById(_ context.Context, req *pb.GetWatcherByIdRequest) (*pb.Watcher, error) {
	w, err := s.fileWatcherRepository.GetWatcherById(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher with id %d not found", req.Id)
	}
	return toProto(w), nil
}

func (s *WatcherService) GetWatcherByName(_ context.Context, req *pb.GetWatcherByNameRequest) (*pb.Watcher, error) {
	w, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}
	return toProto(w), nil
}

func (s *WatcherService) ListWatchers(_ context.Context, _ *pb.ListWatchersRequest) (*pb.ListWatchersResponse, error) {
	watchers, err := s.fileWatcherRepository.ListWatchers()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list watchers: %v", err)
	}
	resp := &pb.ListWatchersResponse{}
	for _, w := range watchers {
		resp.Watchers = append(resp.Watchers, toProto(w))
	}
	return resp, nil
}

func (s *WatcherService) UpdateWatcher(_ context.Context, req *pb.UpdateWatcherRequest) (*pb.Watcher, error) {
	if req.Id < 1 {
		return nil, status.Error(codes.InvalidArgument, "id is invalid (id must be a positive integer)")
	}
	if req.Name == nil && req.SourcePath == nil {
		return nil, status.Error(codes.InvalidArgument, "at least one of name or source_path must be provided")
	}
	w, err := s.fileWatcherRepository.UpdateWatcher(req.Id, req.Name, req.SourcePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update watcher: %v", err)
	}
	return toProto(w), nil
}

func (s *WatcherService) DeleteWatcher(_ context.Context, req *pb.DeleteWatcherRequest) (*pb.DeleteWatcherResponse, error) {
	_, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}
	if err := s.fileWatcherRepository.DeleteWatcher(req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete watcher: %v", err)
	}
	return &pb.DeleteWatcherResponse{Success: true}, nil
}

// SyncWatcher walks the watcher's source_path, diffs against the current
// unflushed watched_files in the DB, and adds/removes entries accordingly.
func (s *WatcherService) SyncWatcher(_ context.Context, req *pb.SyncWatcherRequest) (*pb.SyncWatcherResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	w, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}

	filters, err := s.filterRepository.GetFiltersForWatcher(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load filters: %v", err)
	}

	// Walk the source directory and collect all files that pass the filters.
	onDisk := make(map[string]struct{})
	walkErr := filepath.WalkDir(w.SourcePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if Evaluate(filters, path) {
			onDisk[path] = struct{}{}
		}
		return nil
	})
	if walkErr != nil {
		return nil, status.Errorf(codes.Internal, "walk source path: %v", walkErr)
	}

	// Load current unflushed watched files from DB.
	existing, err := s.fileRepository.ListWatchedFiles(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list watched files: %v", err)
	}
	inDB := make(map[string]struct{}, len(existing))
	for _, f := range existing {
		inDB[f.FilePath] = struct{}{}
	}

	var addedFiles, removedFiles []string

	// Add files that are on disk but not in the DB.
	for path := range onDisk {
		if _, exists := inDB[path]; !exists {
			if _, err := s.fileRepository.AddWatchedFile(req.Name, path, false); err != nil {
				s.logger.Error("sync: error adding file", "watcher", req.Name, "path", path, "err", err)
			} else {
				addedFiles = append(addedFiles, path)
			}
		}
	}

	// Remove DB entries for files no longer on disk.
	for path := range inDB {
		if _, exists := onDisk[path]; !exists {
			if err := s.fileRepository.RemoveWatchedFile(req.Name, path); err != nil {
				s.logger.Error("sync: error removing file", "watcher", req.Name, "path", path, "err", err)
			} else {
				removedFiles = append(removedFiles, path)
			}
		}
	}

	s.logger.Info("sync complete", "watcher", req.Name, "added", len(addedFiles), "removed", len(removedFiles))

	return &pb.SyncWatcherResponse{
		AddedCount:   int32(len(addedFiles)),
		RemovedCount: int32(len(removedFiles)),
		AddedFiles:   addedFiles,
		RemovedFiles: removedFiles,
	}, nil
}

func toProto(w *database.FileWatcher) *pb.Watcher {
	return &pb.Watcher{
		Id:         w.ID,
		Name:       w.Name,
		SourcePath: w.SourcePath,
		CreatedAt:  timestamppb.New(w.CreatedAt),
		UpdatedAt:  timestamppb.New(w.UpdatedAt),
	}
}

// ensure WatcherService satisfies the generated interface at compile time.
var _ pb.FileWatcherServiceServer = (*WatcherService)(nil)
