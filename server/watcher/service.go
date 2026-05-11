package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"google.golang.org/grpc"
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
	logger                *slog.Logger
	// syncJobOpts are forwarded to every SyncJob created by this service.
	// Used in tests to inject a local RemoteFS and bypass SSH.
	syncJobOpts []SyncJobOption
	// syncLocks holds a *sync.Mutex per watcher name.  It ensures that only
	// one sync (SyncWatcher or StreamSyncWatcher) runs at a time for a given
	// watcher.  Entries are created on first use and never removed.
	syncLocks sync.Map
}

// WatcherServiceOption is a functional option for WatcherService.
type WatcherServiceOption func(*WatcherService)

// WithSyncJobOptions forwards the given SyncJobOptions to every SyncJob
// created by this service.  Intended for use in tests.
func WithSyncJobOptions(opts ...SyncJobOption) WatcherServiceOption {
	return func(s *WatcherService) {
		s.syncJobOpts = append(s.syncJobOpts, opts...)
	}
}

func NewManagerService(fileWatcherRepository database.FileWatcherRepository, fileRepository database.FileRepository, machineRepository database.MachineRepository, logger *slog.Logger, opts ...WatcherServiceOption) *WatcherService {
	svc := &WatcherService{
		fileWatcherRepository: fileWatcherRepository,
		fileRepository:        fileRepository,
		machineRepository:     machineRepository,
		logger:                logger,
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// tryAcquireSyncLock attempts to acquire the per-watcher sync lock for name.
// If the lock is free it is acquired and the caller must invoke the returned
// unlock function when the sync is complete.  If the lock is already held by
// another sync, ok is false and unlock is nil.
func (s *WatcherService) tryAcquireSyncLock(name string) (unlock func(), ok bool) {
	v, _ := s.syncLocks.LoadOrStore(name, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	if !mu.TryLock() {
		return nil, false
	}
	return mu.Unlock, true
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
	s.logger.Debug("creating sync job to add existing files", "machine_name", w.MachineName, "watcher_name", w.Name)
	syncJob := NewSyncJob(logger, w, machine, fileRepo, s.fileWatcherRepository, s.syncJobOpts...)
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
func (s *WatcherService) SyncWatcher(_ context.Context, req *pb.SyncWatcherRequest) (*pb.SyncWatcherResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	w, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}

	callerMachine, err := s.machineRepository.GetMachineByName(w.MachineName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "machine %q not found: %v", w.MachineName, err)
	}

	unlock, ok := s.tryAcquireSyncLock(req.Name)
	if !ok {
		return nil, status.Errorf(codes.AlreadyExists, "sync already in progress for watcher %q", req.Name)
	}
	defer unlock()

	syncJob := NewSyncJob(s.logger, w, callerMachine, s.fileRepository, s.fileWatcherRepository, s.syncJobOpts...)

	result, err := syncJob.Run(false)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sync watcher: %v", err)
	}

	return &pb.SyncWatcherResponse{
		AddedCount:   int64(result.AddedCount),
		RemovedCount: int64(result.RemovedCount),
		AddedFiles:   result.AddedFiles.Items(),
		RemovedFiles: result.RemovedFiles,
	}, nil
}

// StreamSyncWatcher is the server-streaming variant of SyncWatcher.
// It sends LOG events at each major step of the sync process and a final
// RESULT event containing the SyncWatcherResponse.
func (s *WatcherService) StreamSyncWatcher(req *pb.SyncWatcherRequest, stream grpc.ServerStreamingServer[pb.SyncWatcherEvent]) error {
	if req.Name == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}

	w, err := s.fileWatcherRepository.GetWatcherByName(req.Name)
	if err != nil {
		return status.Errorf(codes.NotFound, "watcher %s not found", req.Name)
	}

	callerMachine, err := s.machineRepository.GetMachineByName(w.MachineName)
	if err != nil {
		return status.Errorf(codes.Internal, "machine %q not found: %v", w.MachineName, err)
	}

	unlock, ok := s.tryAcquireSyncLock(req.Name)
	if !ok {
		return status.Errorf(codes.AlreadyExists, "sync already in progress for watcher %q", req.Name)
	}
	defer unlock()

	// sendLog forwards a human-readable progress message to the client as a
	// LOG event.  Errors are returned so the caller can abort early.
	sendLog := func(msg string) error {
		return stream.Send(&pb.SyncWatcherEvent{
			Type:    pb.SyncWatcherEvent_LOG,
			Message: msg,
		})
	}

	syncJob := NewSyncJob(
		s.logger, w, callerMachine,
		s.fileRepository, s.fileWatcherRepository,
		append(s.syncJobOpts, WithLogCallback(func(msg string) {
			// Best-effort: ignore send errors inside the callback; the Run()
			// caller will surface any stream error through the returned error.
			_ = sendLog(msg)
		}))...,
	)

	result, err := syncJob.Run(false)
	if err != nil {
		return status.Errorf(codes.Internal, "sync watcher: %v", err)
	}

	// Send a final summary LOG before the RESULT event.
	if err := sendLog(fmt.Sprintf("sync complete: %d file(s) added, %d file(s) removed", result.AddedCount, result.RemovedCount)); err != nil {
		return status.Errorf(codes.Internal, "stream send: %v", err)
	}

	// Send the RESULT event — this is the only message that carries the full
	// SyncWatcherResponse.
	if err := stream.Send(&pb.SyncWatcherEvent{
		Type: pb.SyncWatcherEvent_RESULT,
		Result: &pb.SyncWatcherResponse{
			AddedCount:   int64(result.AddedCount),
			RemovedCount: int64(result.RemovedCount),
			AddedFiles:   result.AddedFiles.Items(),
			RemovedFiles: result.RemovedFiles,
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "stream send result: %v", err)
	}

	return nil
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
