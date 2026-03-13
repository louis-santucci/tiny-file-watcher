package filter

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
)

// FilterRepository defines the persistence operations used by FilterService.
type FilterRepository interface {
	AddFilter(watcherName, ruleType, patternType, pattern string) (*database.WatcherFilter, error)
	GetFiltersForWatcher(watcherName string) ([]*database.WatcherFilter, error)
	ListFilters() ([]*database.WatcherFilter, error)
	DeleteFilter(id int64) error
}

// FilterService implements the WatcherFilterService gRPC server.
type FilterService struct {
	pb.UnimplementedWatcherFilterServiceServer
	repo   FilterRepository
	logger *slog.Logger
}

func NewFilterService(repo FilterRepository, logger *slog.Logger) *FilterService {
	return &FilterService{repo: repo, logger: logger}
}

func (s *FilterService) AddFilter(_ context.Context, req *pb.AddFilterRequest) (*pb.WatcherFilter, error) {
	if req.WatcherName == "" || req.RuleType == "" || req.PatternType == "" || req.Pattern == "" {
		return nil, status.Error(codes.InvalidArgument, "watcher_name, rule_type, pattern_type and pattern are required")
	}
	if req.RuleType != "include" && req.RuleType != "exclude" {
		return nil, status.Error(codes.InvalidArgument, "rule_type must be 'include' or 'exclude'")
	}
	if req.PatternType != "extension" && req.PatternType != "name" && req.PatternType != "glob" {
		return nil, status.Error(codes.InvalidArgument, "pattern_type must be 'extension', 'name', or 'glob'")
	}
	f, err := s.repo.AddFilter(req.WatcherName, req.RuleType, req.PatternType, req.Pattern)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add filter: %v", err)
	}
	return toProto(f), nil
}

func (s *FilterService) ListFilters(_ context.Context, req *pb.ListFiltersRequest) (*pb.ListFiltersResponse, error) {
	var (
		filters []*database.WatcherFilter
		err     error
	)
	if req.WatcherName != "" {
		filters, err = s.repo.GetFiltersForWatcher(req.WatcherName)
	} else {
		filters, err = s.repo.ListFilters()
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list filters: %v", err)
	}
	resp := &pb.ListFiltersResponse{}
	for _, f := range filters {
		resp.Filters = append(resp.Filters, toProto(f))
	}
	return resp, nil
}

func (s *FilterService) DeleteFilter(_ context.Context, req *pb.DeleteFilterRequest) (*pb.DeleteFilterResponse, error) {
	if req.Id < 1 {
		return nil, status.Error(codes.InvalidArgument, "id must be a positive integer")
	}
	if err := s.repo.DeleteFilter(req.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "delete filter: %v", err)
	}
	return &pb.DeleteFilterResponse{Success: true}, nil
}

func toProto(f *database.WatcherFilter) *pb.WatcherFilter {
	return &pb.WatcherFilter{
		Id:          f.ID,
		WatcherName: f.WatcherName,
		RuleType:    f.RuleType,
		PatternType: f.PatternType,
		Pattern:     f.Pattern,
	}
}

// Compile-time assertion.
var _ pb.WatcherFilterServiceServer = (*FilterService)(nil)
