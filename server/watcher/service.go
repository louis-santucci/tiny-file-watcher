package watcher

import (
	"context"
	"log/slog"
	"tiny-file-watcher/server/config"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
)

// WatcherService implements the FileWatcherService gRPC server.
type WatcherService struct {
	pb.UnimplementedFileWatcherServiceServer
	fileWatcherRepository database.FileWatcherRepository
	fileRepository        database.FileRepository
	machineRepository     database.MachineRepository
	transactor            database.Transactor
	logger                *slog.Logger
	sshConfig             *config.SSHConfig
}

func NewManagerService(fileWatcherRepository database.FileWatcherRepository, fileRepository database.FileRepository, machineRepository database.MachineRepository, logger *slog.Logger, sshConfig *config.SSHConfig, transactor database.Transactor) *WatcherService {
	return &WatcherService{
		fileWatcherRepository: fileWatcherRepository,
		fileRepository:        fileRepository,
		machineRepository:     machineRepository,
		logger:                logger,
		sshConfig:             sshConfig,
		transactor:            transactor,
	}
}

func (s *WatcherService) CreateWatcher(_ context.Context, req *pb.CreateWatcherRequest) (*pb.Watcher, error) {
	if req.Name == "" || req.SourcePath == "" {
		return nil, status.Error(codes.InvalidArgument, "name and source_path are required")
	}
	if req.MachineName == "" {
		return nil, status.Error(codes.InvalidArgument, "machine_name is required: initialize this machine first with 'tfw machine create'")
	}
	w, err := s.fileWatcherRepository.CreateWatcher(req.Name, req.SourcePath, req.MachineName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create watcher: %v", err)
	}
	if req.FlushExisting {
		s.AddExistingFiles(w, s.fileRepository, s.logger)
	}

	return toProto(w), nil
}

func (s *WatcherService) AddExistingFiles(w *database.FileWatcher, fileRepo database.FileRepository, logger *slog.Logger) {
	machine, err := s.machineRepository.GetMachineByName(w.MachineName)
	if err != nil {
		logger.Error("fetch machine for watcher", "machine_name", w.MachineName, "error", err)
	}
	syncJob := NewSyncJob(logger, w, machine, s.sshConfig, nil, fileRepo, s.fileWatcherRepository, s.transactor)
	if _, err := syncJob.Run(true); err != nil {
		logger.Error("add existing files for watcher", "watcher_name", w.Name, "error", err)
	}
}

func (s *WatcherService) ListWatchedFiles(_ context.Context, req *pb.ListWatchedFilesRequest) (*pb.ListWatchedFilesResponse, error) {
	if req.WatcherName == "" {
		return nil, status.Error(codes.InvalidArgument, "watcher_name is required")
	}
	files, err := s.fileRepository.ListWatchedFiles(req.WatcherName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list watched files: %v", err)
	}
	resp := &pb.ListWatchedFilesResponse{}
	for _, f := range files {
		resp.Files = append(resp.Files, &pb.WatchedFile{
			FilePath:   f.FilePath,
			Flushed:    f.Flushed,
			DetectedAt: timestamppb.New(f.DetectedAt),
		})
	}
	return resp, nil
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

func (s *WatcherService) ListWatchers(_ context.Context, req *pb.ListWatchersRequest) (*pb.ListWatchersResponse, error) {
	var (
		watchers []*database.FileWatcher
		err      error
	)
	if req.MachineName != nil && *req.MachineName != "" {
		watchers, err = s.fileWatcherRepository.ListWatchersByMachine(*req.MachineName)
	} else {
		watchers, err = s.fileWatcherRepository.ListWatchers()
	}
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
// It validates that the caller's machine (identified by token) owns the watcher.
func (s *WatcherService) SyncWatcher(_ context.Context, req *pb.SyncWatcherRequest) (*pb.SyncWatcherResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	w, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}

	// Verify the caller's machine owns this watcher.
	callerMachine, err := s.machineRepository.GetMachineByToken(req.Token)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "machine with token %q is not registered", req.Token)
	}
	if callerMachine.Name != w.MachineName {
		return nil, status.Errorf(codes.PermissionDenied,
			"watcher %q belongs to machine %q, but request comes from machine %q",
			req.Name, w.MachineName, callerMachine.Name)
	}

	remoteMachine, err := s.machineRepository.GetMachineByName(w.MachineName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetch machine for watcher: %v", err)
	}

	var publicKey ssh.PublicKey
	syncJob := NewSyncJob(s.logger, w, remoteMachine, s.sshConfig, publicKey, s.fileRepository, s.fileWatcherRepository, s.transactor)

	result, err := syncJob.Run(false)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sync watcher: %v", err)
	}

	return &pb.SyncWatcherResponse{
		AddedCount:   int64(result.AddedCount),
		RemovedCount: int64(result.RemovedCount),
		AddedFiles:   result.AddedFiles,
		RemovedFiles: result.RemovedFiles,
	}, nil
}

func toProto(w *database.FileWatcher) *pb.Watcher {
	return &pb.Watcher{
		Id:          w.ID,
		Name:        w.Name,
		SourcePath:  w.SourcePath,
		MachineName: w.MachineName,
		CreatedAt:   timestamppb.New(w.CreatedAt),
		UpdatedAt:   timestamppb.New(w.UpdatedAt),
	}
}

// ensure WatcherService satisfies the generated interface at compile time.
var _ pb.FileWatcherServiceServer = (*WatcherService)(nil)
