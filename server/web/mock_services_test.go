package web

import (
	"context"

	pb "tiny-file-watcher/gen/grpc"

	"github.com/stretchr/testify/mock"
)

// mockWatcherService implements watcherService.
type mockWatcherService struct {
	mock.Mock
}

func (m *mockWatcherService) ListWatchers(ctx context.Context, req *pb.ListWatchersRequest) (*pb.ListWatchersResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.ListWatchersResponse), args.Error(1)
}

func (m *mockWatcherService) ToggleWatcher(ctx context.Context, req *pb.ToggleWatcherRequest) (*pb.Watcher, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.Watcher), args.Error(1)
}

// mockFlushService implements flushService.
type mockFlushService struct {
	mock.Mock
}

func (m *mockFlushService) ListPendingFiles(ctx context.Context, req *pb.ListPendingFilesRequest) (*pb.ListPendingFilesResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.ListPendingFilesResponse), args.Error(1)
}

func (m *mockFlushService) FlushWatcher(ctx context.Context, req *pb.FlushWatcherRequest) (*pb.FlushWatcherResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.FlushWatcherResponse), args.Error(1)
}

// mockRedirectionService implements redirectionService.
type mockRedirectionService struct {
	mock.Mock
}

func (m *mockRedirectionService) GetFileRedirection(ctx context.Context, req *pb.GetFileRedirectionRequest) (*pb.FileRedirection, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.FileRedirection), args.Error(1)
}

// mockFilterService implements filterService.
type mockFilterService struct {
	mock.Mock
}

func (m *mockFilterService) ListFilters(ctx context.Context, req *pb.ListFiltersRequest) (*pb.ListFiltersResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pb.ListFiltersResponse), args.Error(1)
}
