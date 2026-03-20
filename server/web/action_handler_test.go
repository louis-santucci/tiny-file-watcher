package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "tiny-file-watcher/gen/grpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- handleToggle ---

func TestHandleToggle_Success(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ToggleWatcher", mock.Anything, &pb.ToggleWatcherRequest{Name: "alpha"}).
		Return(&pb.Watcher{Name: "alpha", SourcePath: "/src/alpha", Enabled: true}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/toggle", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "alpha")
	assert.Contains(t, w.Body.String(), "Enabled")
	watcherSvc.AssertExpectations(t)
}

func TestHandleToggle_DisabledWatcher(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ToggleWatcher", mock.Anything, &pb.ToggleWatcherRequest{Name: "beta"}).
		Return(&pb.Watcher{Name: "beta", SourcePath: "/src/beta", Enabled: false}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/beta/toggle", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Disabled")
	assert.Contains(t, w.Body.String(), "Enable")
}

func TestHandleToggle_ServiceError(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ToggleWatcher", mock.Anything, mock.Anything).
		Return(nil, errors.New("toggle failed"))

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/toggle", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "toggle failed")
}

// --- handleFlush ---

func TestHandleFlush_Success(t *testing.T) {
	flushSvc := &mockFlushService{}
	flushSvc.On("FlushWatcher", mock.Anything, &pb.FlushWatcherRequest{Name: "alpha"}).
		Return(&pb.FlushWatcherResponse{Success: true}, nil)
	flushSvc.On("ListPendingFiles", mock.Anything, &pb.ListPendingFilesRequest{Name: "alpha"}).
		Return(&pb.ListPendingFilesResponse{Files: []*pb.WatchedFile{{FileName: "report.csv"}}}, nil)

	h, err := New(&mockWatcherService{}, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/flush", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "report.csv")
	flushSvc.AssertExpectations(t)
}

func TestHandleFlush_EmptyPendingFiles(t *testing.T) {
	flushSvc := &mockFlushService{}
	flushSvc.On("FlushWatcher", mock.Anything, &pb.FlushWatcherRequest{Name: "alpha"}).
		Return(&pb.FlushWatcherResponse{Success: true}, nil)
	flushSvc.On("ListPendingFiles", mock.Anything, &pb.ListPendingFilesRequest{Name: "alpha"}).
		Return(&pb.ListPendingFilesResponse{Files: nil}, nil)

	h, err := New(&mockWatcherService{}, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/flush", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No pending files")
}

func TestHandleFlush_NotFoundFlushIsIgnored(t *testing.T) {
	flushSvc := &mockFlushService{}
	// FlushWatcher returns NotFound — the handler should treat this as a no-op.
	notFoundErr := status.Errorf(codes.NotFound, "no redirection configured")
	flushSvc.On("FlushWatcher", mock.Anything, mock.Anything).
		Return(nil, notFoundErr)
	flushSvc.On("ListPendingFiles", mock.Anything, &pb.ListPendingFilesRequest{Name: "alpha"}).
		Return(&pb.ListPendingFilesResponse{Files: nil}, nil)

	h, err := New(&mockWatcherService{}, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/flush", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandleFlush_FlushServiceError(t *testing.T) {
	flushSvc := &mockFlushService{}
	flushSvc.On("FlushWatcher", mock.Anything, mock.Anything).
		Return(nil, errors.New("internal flush error"))

	h, err := New(&mockWatcherService{}, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/flush", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal flush error")
}

func TestHandleFlush_ListPendingFilesError(t *testing.T) {
	flushSvc := &mockFlushService{}
	flushSvc.On("FlushWatcher", mock.Anything, mock.Anything).
		Return(&pb.FlushWatcherResponse{Success: true}, nil)
	flushSvc.On("ListPendingFiles", mock.Anything, mock.Anything).
		Return(nil, errors.New("db error"))

	h, err := New(&mockWatcherService{}, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/flush", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "db error")
}
