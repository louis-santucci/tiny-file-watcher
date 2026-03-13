package watcher

import (
	"context"
	"log/slog"

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
	manager               WatcherManager
	logger                *slog.Logger
}

func NewManagerService(fileWatcherRepository FileWatcherRepository, fileRepository FileRepository, mgr WatcherManager, logger *slog.Logger) *WatcherService {
	return &WatcherService{fileWatcherRepository: fileWatcherRepository, manager: mgr, fileRepository: fileRepository, logger: logger}
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
	watcher, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}
	// Stop goroutine before deleting from DB.
	key := WatcherKey{Id: watcher.ID, Name: watcher.Name}
	s.manager.Stop(key)
	if err := s.fileWatcherRepository.DeleteWatcher(req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete watcher: %v", err)
	}
	return &pb.DeleteWatcherResponse{Success: true}, nil
}

func (s *WatcherService) ToggleWatcher(_ context.Context, req *pb.ToggleWatcherRequest) (*pb.Watcher, error) {
	w, err := s.fileWatcherRepository.ToggleWatcher(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "toggle watcher: %v", err)
	}
	key := WatcherKey{Id: w.ID, Name: w.Name}
	if w.Enabled {
		if err := s.manager.Start(key, w.SourcePath); err != nil {
			// Roll back toggle on failure.
			_, _ = s.fileWatcherRepository.ToggleWatcher(req.Name)
			return nil, status.Errorf(codes.Internal, "start watcher goroutine: %v", err)
		}
	} else {
		s.manager.Stop(key)
	}
	return toProto(w), nil
}

func toProto(w *database.FileWatcher) *pb.Watcher {
	return &pb.Watcher{
		Id:         w.ID,
		Name:       w.Name,
		SourcePath: w.SourcePath,
		Enabled:    w.Enabled,
		CreatedAt:  timestamppb.New(w.CreatedAt),
		UpdatedAt:  timestamppb.New(w.UpdatedAt),
	}
}

// ensure WatcherService satisfies the generated interface at compile time.
var _ pb.FileWatcherServiceServer = (*WatcherService)(nil)
