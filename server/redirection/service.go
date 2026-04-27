package redirection

import (
	"context"
	"log/slog"
	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RedirectionService struct {
	pb.UnimplementedFileRedirectionServiceServer
	fileWatcherRepository database.FileWatcherRepository
	fileRepository        database.FileRepository
	redirectionRepository database.RedirectionRepository
	machineRepository     database.MachineRepository
	logger                *slog.Logger
}

func NewRedirectionService(fileWatcherRepository database.FileWatcherRepository, fileRepository database.FileRepository, redirectionRepository database.RedirectionRepository, machineRepository database.MachineRepository, logger *slog.Logger) *RedirectionService {
	return &RedirectionService{
		fileWatcherRepository: fileWatcherRepository,
		fileRepository:        fileRepository,
		redirectionRepository: redirectionRepository,
		machineRepository:     machineRepository,
		logger:                logger,
	}
}

func (s *RedirectionService) CreateFileRedirection(_ context.Context, req *pb.CreateFileRedirectionRequest) (*pb.FileRedirection, error) {
	if req.WatcherName == "" || req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "watcher_name and target_path are required")
	}
	if req.TargetMachineName == "" {
		return nil, status.Error(codes.InvalidArgument, "target_machine_name is required")
	}

	// Verify the target machine exists.
	if _, err := s.machineRepository.GetMachineByName(req.TargetMachineName); err != nil {
		return nil, status.Errorf(codes.NotFound, "target machine %q not found", req.TargetMachineName)
	}

	redirection, err := s.redirectionRepository.AddRedirection(req.WatcherName, req.TargetPath, req.TargetMachineName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add redirection: %v", err)
	}
	return toProto(redirection), nil
}

func (s *RedirectionService) GetFileRedirection(_ context.Context, req *pb.GetFileRedirectionRequest) (*pb.FileRedirection, error) {
	redirection, err := s.redirectionRepository.GetRedirection(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "redirection for watcher %s not found", req.Name)
	}
	return toProto(redirection), nil
}

func (s *RedirectionService) UpdateFileRedirection(_ context.Context, req *pb.UpdateFileRedirectionRequest) (*pb.FileRedirection, error) {
	if req.WatcherName == "" || req.TargetPath == nil {
		return nil, status.Error(codes.InvalidArgument, "watcher_name and target_path are required")
	}
	redirection, err := s.redirectionRepository.UpdateRedirection(req.WatcherName, req.TargetPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "update redirection: %v", err)
	}
	return toProto(redirection), nil
}

func (s *RedirectionService) DeleteFileRedirection(_ context.Context, req *pb.DeleteFileRedirectionRequest) (*pb.DeleteFileRedirectionResponse, error) {
	if err := s.redirectionRepository.RemoveRedirection(req.WatcherName); err != nil {
		return nil, status.Errorf(codes.Internal, "delete redirection: %v", err)
	}
	return &pb.DeleteFileRedirectionResponse{Success: true}, nil
}

func toProto(r *database.FileRedirection) *pb.FileRedirection {
	return &pb.FileRedirection{
		WatcherName:       r.WatcherName,
		TargetPath:        r.TargetPath,
		TargetMachineName: r.TargetMachineName,
		CreatedAt:         timestamppb.New(r.CreatedAt),
		UpdatedAt:         timestamppb.New(r.UpdatedAt),
	}
}
