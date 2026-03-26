package web

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	pb "tiny-file-watcher/gen/grpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// buildUploadRequest constructs a multipart POST to /watchers/{name}/upload.
// folder may be empty (field is omitted when blank).
func buildUploadRequest(t *testing.T, watcherName, folder, filename, content string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if folder != "" {
		require.NoError(t, mw.WriteField("folder", folder))
	}

	fw, err := mw.CreateFormFile("files", filename)
	require.NoError(t, err)
	_, err = fw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/watchers/"+watcherName+"/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func enabledWatcher(name, sourcePath string) *pb.Watcher {
	return &pb.Watcher{Name: name, SourcePath: sourcePath}
}

// --- resolveUploadDir ---

func TestResolveUploadDir_NoFolder(t *testing.T) {
	dir, err := resolveUploadDir("/src/path", "")
	require.NoError(t, err)
	assert.Equal(t, "/src/path", dir)
}

func TestResolveUploadDir_WithFolder(t *testing.T) {
	dir, err := resolveUploadDir("/src/path", "toto")
	require.NoError(t, err)
	assert.Equal(t, "/src/path/toto", dir)
}

func TestResolveUploadDir_TraversalDotDot(t *testing.T) {
	_, err := resolveUploadDir("/src/path", "..")
	assert.Error(t, err)
}

func TestResolveUploadDir_TraversalWithSlash(t *testing.T) {
	_, err := resolveUploadDir("/src/path", "sub/dir")
	assert.Error(t, err)
}

func TestResolveUploadDir_TraversalBackslash(t *testing.T) {
	_, err := resolveUploadDir("/src/path", `sub\dir`)
	assert.Error(t, err)
}

func TestResolveUploadDir_TraversalAbsolute(t *testing.T) {
	_, err := resolveUploadDir("/src/path", "/etc/passwd")
	assert.Error(t, err)
}

// --- handleUpload integration with handler ---

func TestHandleUpload_NoFolder_FileSavedToSourcePath(t *testing.T) {
	srcDir := t.TempDir()

	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{enabledWatcher("alpha", srcDir)}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	req := buildUploadRequest(t, "alpha", "", "hello.txt", "world")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "hello.txt")

	data, err := os.ReadFile(filepath.Join(srcDir, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "world", string(data))
}

func TestHandleUpload_WithFolder_FileSavedToSubfolder(t *testing.T) {
	srcDir := t.TempDir()

	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{enabledWatcher("alpha", srcDir)}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	req := buildUploadRequest(t, "alpha", "toto", "toto.mp3", "audio")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "toto.mp3")

	// File must be at srcDir/toto/toto.mp3
	data, err := os.ReadFile(filepath.Join(srcDir, "toto", "toto.mp3"))
	require.NoError(t, err)
	assert.Equal(t, "audio", string(data))

	// File must NOT be at srcDir/toto.mp3
	_, statErr := os.Stat(filepath.Join(srcDir, "toto.mp3"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestHandleUpload_SubfolderCreatedIfMissing(t *testing.T) {
	srcDir := t.TempDir()

	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{enabledWatcher("alpha", srcDir)}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	req := buildUploadRequest(t, "alpha", "newdir", "file.txt", "content")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	info, err := os.Stat(filepath.Join(srcDir, "newdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestHandleUpload_InvalidFolder_Rejected(t *testing.T) {
	srcDir := t.TempDir()

	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{enabledWatcher("alpha", srcDir)}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	req := buildUploadRequest(t, "alpha", "../escape", "file.txt", "content")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid folder")
}

func TestHandleUpload_WatcherNotFound(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	req := buildUploadRequest(t, "missing", "", "file.txt", "content")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandleUpload_NoFiles(t *testing.T) {
	srcDir := t.TempDir()

	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(&pb.ListWatchersResponse{Watchers: []*pb.Watcher{enabledWatcher("alpha", srcDir)}}, nil)

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	// Send a multipart form with only the folder field, no files.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("folder", "toto"))
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/watchers/alpha/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "No files selected")
}

func TestHandleUpload_ListWatchersError(t *testing.T) {
	watcherSvc := &mockWatcherService{}
	watcherSvc.On("ListWatchers", mock.Anything, mock.Anything).
		Return(nil, errors.New("db unavailable"))

	h, err := New(watcherSvc, &mockFlushService{}, &mockRedirectionService{}, &mockFilterService{}, OIDCConfig{})
	require.NoError(t, err)

	req := buildUploadRequest(t, "alpha", "", "file.txt", "content")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "db unavailable")
}
