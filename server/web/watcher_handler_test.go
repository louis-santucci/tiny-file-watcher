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
)

// --- handleDashboard ---

func TestHandleDashboard_Success(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	flushSvc := &mockFlushService{}

	watchers := []*pb.Watcher{
		{Name: "alpha", SourcePath: "/src/alpha", Enabled: true},
		{Name: "beta", SourcePath: "/src/beta", Enabled: false},
	}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: watchers}, nil)

	flushSvc.On("ListPendingFiles", mock.Anything, &pb.ListPendingFilesRequest{Name: "alpha"}).
		Return(&pb.ListPendingFilesResponse{Files: []*pb.WatchedFile{{FileName: "a.txt"}, {FileName: "b.txt"}}}, nil)
	flushSvc.On("ListPendingFiles", mock.Anything, &pb.ListPendingFilesRequest{Name: "beta"}).
		Return(&pb.ListPendingFilesResponse{Files: nil}, nil)

	h, err := New(watcherSvc, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	watcherSvc.AssertExpectations(t)
	flushSvc.AssertExpectations(t)
}

func TestHandleDashboard_ListWatchersError(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(nil, errors.New("db down"))

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "db down")
}

func TestHandleDashboard_PendingFilesErrorIsIgnored(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	flushSvc := &mockFlushService{}

	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{{Name: "alpha", Enabled: true}}}, nil)
	// ListPendingFiles returns an error — the handler should silently skip it.
	flushSvc.On("ListPendingFiles", mock.Anything, mock.Anything).
		Return(nil, errors.New("flush unavailable"))

	h, err := New(watcherSvc, flushSvc, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// --- handleWatcherList ---

func TestHandleWatcherList_Success(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watchers := []*pb.Watcher{
		{Name: "alpha", SourcePath: "/src/alpha", Enabled: true},
		{Name: "beta", SourcePath: "/src/beta", Enabled: false},
	}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: watchers}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/watchers", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	watcherSvc.AssertExpectations(t)
}

func TestHandleWatcherList_ServiceError(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(nil, errors.New("connection refused"))

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/watchers", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "connection refused")
}

// --- handleWatcherDetail ---

func TestHandleWatcherDetail_Success(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	flushSvc := &mockFlushService{}
	redirectSvc := &mockRedirectionService{}
	filterSvc := &mockFilterService{}

	watchers := []*pb.Watcher{{Name: "alpha", SourcePath: "/src/alpha", Enabled: true}}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: watchers}, nil)

	redirectSvc.On("GetFileRedirection", mock.Anything, &pb.GetFileRedirectionRequest{Name: "alpha"}).
		Return(&pb.FileRedirection{WatcherName: "alpha", TargetPath: "/dst/alpha"}, nil)

	filterSvc.On("ListFilters", mock.Anything, &pb.ListFiltersRequest{WatcherName: "alpha"}).
		Return(&pb.ListFiltersResponse{Filters: []*pb.WatcherFilter{
			{WatcherName: "alpha", RuleType: "include", PatternType: "extension", Pattern: ".go"},
		}}, nil)

	flushSvc.On("ListPendingFiles", mock.Anything, &pb.ListPendingFilesRequest{Name: "alpha"}).
		Return(&pb.ListPendingFilesResponse{Files: []*pb.WatchedFile{{FileName: "main.go"}}}, nil)

	h, err := New(watcherSvc, flushSvc, redirectSvc, filterSvc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/watchers/alpha", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	watcherSvc.AssertExpectations(t)
	redirectSvc.AssertExpectations(t)
	filterSvc.AssertExpectations(t)
	flushSvc.AssertExpectations(t)
}

func TestHandleWatcherDetail_NotFound(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{{Name: "alpha"}}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/watchers/does-not-exist", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "watcher not found")
}

func TestHandleWatcherDetail_ListWatchersError(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(nil, errors.New("service unavailable"))

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/watchers/alpha", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "service unavailable")
}

func TestHandleWatcherDetail_OptionalServicesErrorsAreIgnored(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	flushSvc := &mockFlushService{}
	redirectSvc := &mockRedirectionService{}
	filterSvc := &mockFilterService{}

	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{{Name: "alpha"}}}, nil)
	// Redirection, filters, and pending files all fail — the handler renders anyway.
	redirectSvc.On("GetFileRedirection", mock.Anything, mock.Anything).
		Return(nil, errors.New("no redirection"))
	filterSvc.On("ListFilters", mock.Anything, mock.Anything).
		Return(nil, errors.New("no filters"))
	flushSvc.On("ListPendingFiles", mock.Anything, mock.Anything).
		Return(nil, errors.New("no pending"))

	h, err := New(watcherSvc, flushSvc, redirectSvc, filterSvc)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/watchers/alpha", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
