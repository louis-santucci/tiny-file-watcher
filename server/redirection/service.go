package redirection

import (
	"context"
	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/watcher"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RedirectionService struct {
	pb.UnimplementedFileRedirectionServiceServer
	fileWatcherRepository watcher.FileWatcherRepository
	fileRepository        watcher.FileRepository
	redirectionRepository RedirectionRepository
}

func NewRedirectionService(fileWatcherRepository watcher.FileWatcherRepository, fileRepository watcher.FileRepository) *RedirectionService {
	return &RedirectionService{fileWatcherRepository: fileWatcherRepository, fileRepository: fileRepository}
}

func (s *RedirectionService) AddRedirection(_ context.Context, req *pb.CreateFileRedirectionRequest) (*pb.FileRedirection, error) {
	if req.WatcherName == "" || req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "watcher_name and file_path are required")
	}
	redirection, err := s.redirectionRepository.AddRedirection(req.WatcherName, req.TargetPath, req.AutoFlush)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add redirection: %v", err)
	}
	return toProto(redirection), nil
}

func (s *RedirectionService) GetRedirection(_ context.Context, req *pb.GetFileRedirectionRequest) (*pb.FileRedirection, error) {
	redirection, err := s.redirectionRepository.GetRedirection(req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "redirection for watcher %s not found", req.Name)
	}
	return toProto(redirection), nil
}

func (s *RedirectionService) UpdateFileRedirection(_ context.Context, req *pb.UpdateFileRedirectionRequest) (*pb.FileRedirection, error) {
	if req.WatcherName == "" || req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "watcher_name and file_path are required")
	}
	redirection, err := s.redirectionRepository.UpdateRedirection(req.WatcherName, req.TargetPath, req.AutoFlush)
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
	var createdAt, updatedAt *timestamppb.Timestamp
	createdAt = timestamppb.New(r.CreatedAt)
	updatedAt = timestamppb.New(r.UpdatedAt)
	return &pb.FileRedirection{
		WatcherName: r.WatcherName,
		TargetPath:  r.TargetPath,
		AutoFlush:   r.AutoFlush,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}
