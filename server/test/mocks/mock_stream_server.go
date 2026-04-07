package mocks

import (
	"context"

	pb "tiny-file-watcher/gen/grpc"

	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/metadata"
)

// MockStreamSyncWatcherServer is a testify mock that satisfies the
// grpc.ServerStreamingServer[pb.SyncWatcherEvent] interface used by
// WatcherService.StreamSyncWatcher.
type MockStreamSyncWatcherServer struct {
	mock.Mock
	// Sent collects every event delivered via Send, in order.
	Sent []*pb.SyncWatcherEvent
}

func (m *MockStreamSyncWatcherServer) Send(evt *pb.SyncWatcherEvent) error {
	m.Sent = append(m.Sent, evt)
	args := m.Called(evt)
	return args.Error(0)
}

func (m *MockStreamSyncWatcherServer) Context() context.Context {
	return context.Background()
}

func (m *MockStreamSyncWatcherServer) SendMsg(msg any) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockStreamSyncWatcherServer) RecvMsg(msg any) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockStreamSyncWatcherServer) SendHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockStreamSyncWatcherServer) SetHeader(md metadata.MD) error {
	args := m.Called(md)
	return args.Error(0)
}

func (m *MockStreamSyncWatcherServer) SetTrailer(md metadata.MD) {
	m.Called(md)
}
