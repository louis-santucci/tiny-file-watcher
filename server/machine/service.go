package machine

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
)

// MachineService implements the MachineService gRPC server.
type MachineService struct {
	pb.UnimplementedMachineServiceServer
	repo   MachineRepository
	logger *slog.Logger
}

func NewMachineService(repo MachineRepository, logger *slog.Logger) *MachineService {
	return &MachineService{repo: repo, logger: logger}
}

func (s *MachineService) CreateMachine(_ context.Context, req *pb.InitializeMachineRequest) (*pb.MachineResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}
	if req.Ip == "" {
		return nil, status.Error(codes.InvalidArgument, "ip is required")
	}
	if req.SshUser == "" {
		return nil, status.Error(codes.InvalidArgument, "ssh_user is required")
	}
	if req.SshPrivateKey == "" {
		return nil, status.Error(codes.InvalidArgument, "ssh_key is required")
	}
	sshPort := req.SshPort
	if sshPort == 0 {
		sshPort = 22
	}
	m, err := s.repo.CreateMachine(req.Name, req.Token, req.Ip, sshPort, req.SshUser, req.SshPrivateKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create machine: %v", err)
	}
	s.logger.Info("machine created", "name", m.Name, "token", m.Token, "ip", m.IP, "ssh_port", m.SSHPort)
	return toProto(m), nil
}

func (s *MachineService) GetMachines(_ context.Context, _ *pb.EmptyRequest) (*pb.GetMachinesResponse, error) {
	machines, err := s.repo.ListMachines()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list machines: %v", err)
	}
	resp := &pb.GetMachinesResponse{}
	for _, m := range machines {
		resp.Machines = append(resp.Machines, toProto(m))
	}
	return resp, nil
}

func (s *MachineService) DeleteMachine(_ context.Context, req *pb.DeleteMachineRequest) (*pb.DeleteMachineResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := s.repo.DeleteMachine(req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete machine: %v", err)
	}
	s.logger.Info("machine deleted", "name", req.Name)
	return &pb.DeleteMachineResponse{Success: true}, nil
}

func toProto(m *database.Machine) *pb.MachineResponse {
	return &pb.MachineResponse{
		Token:     m.Token,
		Name:      m.Name,
		CreatedAt: timestamppb.New(m.CreatedAt),
		UpdatedAt: timestamppb.New(m.UpdatedAt),
		Ip:        m.IP,
		SshPort:   m.SSHPort,
	}
}

// Compile-time assertion.
var _ pb.MachineServiceServer = (*MachineService)(nil)
