package watcher_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ctx     = context.Background()
	fixedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

func newWatcher(id int64, name, path string) *database.FileWatcher {
	return &database.FileWatcher{
		ID:         id,
		Name:       name,
		SourcePath: path,
		CreatedAt:  fixedAt,
		UpdatedAt:  fixedAt,
	}
}

func newService(fileWatcherRepository *mocks.MockFileWatcherRepository, fileRepository *mocks.MockFileRepository, filterRepository *mocks.MockFilterRepository) *watcher.WatcherService {
	return watcher.NewManagerService(fileWatcherRepository, fileRepository, filterRepository, testutil.TestLogger())
}

// ── CreateWatcher ─────────────────────────────────────────────────────────────

func TestCreateWatcher_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	w := newWatcher(1, "my-watcher", "/tmp/src")
	fileWatcherRepo.On("CreateWatcher", "my-watcher", "/tmp/src").Return(w, nil)

	resp, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "my-watcher", SourcePath: "/tmp/src"})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.Id)
	assert.Equal(t, "my-watcher", resp.Name)
	assert.Equal(t, "/tmp/src", resp.SourcePath)
	fileWatcherRepo.AssertExpectations(t)
}

func TestCreateWatcher_WithFlushExisting(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	w := newWatcher(1, "my-watcher", "/tmp/src")
	fileWatcherRepo.On("CreateWatcher", "my-watcher", "/tmp/src").Return(w, nil)

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "my-watcher", SourcePath: "/tmp/src", FlushExisting: true})

	assert.NoError(t, err)
	fileWatcherRepo.AssertExpectations(t)
}

func TestCreateWatcher_MissingName(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockFilterRepository{})

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "", SourcePath: "/tmp/src"})

	assertCode(t, err, codes.InvalidArgument)
}

func TestCreateWatcher_MissingSourcePath(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockFilterRepository{})

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "w", SourcePath: ""})

	assertCode(t, err, codes.InvalidArgument)
}

func TestCreateWatcher_DBError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("CreateWatcher", "w", "/tmp").Return(nil, errors.New("db error"))

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "w", SourcePath: "/tmp"})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── GetWatcherById ────────────────────────────────────────────────────────────

func TestGetWatcherById_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	w := newWatcher(42, "w", "/tmp")
	fileWatcherRepo.On("GetWatcherById", int64(42)).Return(w, nil)

	resp, err := svc.GetWatcherById(ctx, &pb.GetWatcherByIdRequest{Id: 42})

	assert.NoError(t, err)
	assert.Equal(t, int64(42), resp.Id)
	fileWatcherRepo.AssertExpectations(t)
}

func TestGetWatcherById_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("GetWatcherById", int64(99)).Return(nil, errors.New("not found"))

	_, err := svc.GetWatcherById(ctx, &pb.GetWatcherByIdRequest{Id: 99})

	assertCode(t, err, codes.NotFound)
	fileWatcherRepo.AssertExpectations(t)
}

// ── GetWatcherByName ──────────────────────────────────────────────────────────

func TestGetWatcherByName_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	w := newWatcher(1, "foo", "/tmp")
	fileWatcherRepo.On("GetWatcherByName", "foo").Return(w, nil)

	resp, err := svc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: "foo"})

	assert.NoError(t, err)
	assert.Equal(t, "foo", resp.Name)
	fileWatcherRepo.AssertExpectations(t)
}

func TestGetWatcherByName_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("GetWatcherByName", "missing").Return(nil, errors.New("not found"))

	_, err := svc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: "missing"})

	assertCode(t, err, codes.NotFound)
	fileWatcherRepo.AssertExpectations(t)
}

// ── ListWatchers ──────────────────────────────────────────────────────────────

func TestListWatchers_Empty(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("ListWatchers").Return([]*database.FileWatcher{}, nil)

	resp, err := svc.ListWatchers(ctx, &pb.ListWatchersRequest{})

	assert.NoError(t, err)
	assert.Empty(t, resp.Watchers)
	fileWatcherRepo.AssertExpectations(t)
}

func TestListWatchers_Populated(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	watchers := []*database.FileWatcher{
		newWatcher(1, "a", "/tmp/a"),
		newWatcher(2, "b", "/tmp/b"),
	}
	fileWatcherRepo.On("ListWatchers").Return(watchers, nil)

	resp, err := svc.ListWatchers(ctx, &pb.ListWatchersRequest{})

	assert.NoError(t, err)
	assert.Len(t, resp.Watchers, 2)
	fileWatcherRepo.AssertExpectations(t)
}

func TestListWatchers_DBError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("ListWatchers").Return(nil, errors.New("db error"))

	_, err := svc.ListWatchers(ctx, &pb.ListWatchersRequest{})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── UpdateWatcher ─────────────────────────────────────────────────────────────

func TestUpdateWatcher_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	w := newWatcher(1, "new-name", "/new/path")
	name := "new-name"
	sourcePath := "/new/path"
	fileWatcherRepo.On("UpdateWatcher", int64(1), &name, &sourcePath).Return(w, nil)

	resp, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 1, Name: &name, SourcePath: &sourcePath})

	assert.NoError(t, err)
	assert.Equal(t, "new-name", resp.Name)
	fileWatcherRepo.AssertExpectations(t)
}

func TestUpdateWatcher_InvalidId(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	name := "n"
	sourcePath := "/p"

	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 0, Name: &name, SourcePath: &sourcePath})

	assertCode(t, err, codes.InvalidArgument)
}

func TestUpdateWatcher_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	name := "n"
	sourcePath := "/new/path"

	fileWatcherRepo.On("UpdateWatcher", int64(727), &name, &sourcePath).Return(nil, errors.New("not found"))
	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 727, Name: &name, SourcePath: &sourcePath})

	assert.Error(t, err)
}
func TestUpdateWatcher_NullParameter(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	var name *string = nil
	newPath := "/new/path"

	w := newWatcher(1, "old-name", "/new/path")
	fileWatcherRepo.On("UpdateWatcher", int64(1), name, &newPath).Return(w, nil)
	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 1, SourcePath: &newPath})

	assert.NoError(t, err)
	assert.Equal(t, "old-name", w.Name)
	assert.Equal(t, "/new/path", w.SourcePath)
}

func TestUpdateWatcher_DBError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	name := "n"
	sourcePath := "/p"

	fileWatcherRepo.On("UpdateWatcher", int64(1), &name, &sourcePath).Return(nil, errors.New("db error"))

	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 1, Name: &name, SourcePath: &sourcePath})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── DeleteWatcher ─────────────────────────────────────────────────────────────

func TestDeleteWatcher_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	w := newWatcher(1, "to-delete", "/tmp")
	fileWatcherRepo.On("GetWatcherByName", "to-delete").Return(w, nil)
	fileWatcherRepo.On("DeleteWatcher", "to-delete").Return(nil)

	resp, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: "to-delete"})

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	fileWatcherRepo.AssertExpectations(t)
}

func TestDeleteWatcher_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("GetWatcherByName", "ghost").Return(nil, errors.New("not found"))

	_, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: "ghost"})

	assertCode(t, err, codes.NotFound)
	fileWatcherRepo.AssertExpectations(t)
}

func TestDeleteWatcher_DBDeleteError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockFilterRepository{})

	w := newWatcher(1, "w", "/tmp")
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	fileWatcherRepo.On("DeleteWatcher", "w").Return(errors.New("db error"))

	_, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: "w"})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── SyncWatcher ───────────────────────────────────────────────────────────────

func TestSyncWatcher_EmptyDir_NoFilesInDB(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.AddedCount)
	assert.Equal(t, int32(0), resp.RemovedCount)
	fileWatcherRepo.AssertExpectations(t)
	fileRepo.AssertExpectations(t)
}

func TestSyncWatcher_NewFilesAdded(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	// Create files on disk.
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	for _, p := range []string{f1, f2} {
		require(t, os.WriteFile(p, []byte("x"), 0o644))
	}

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	fileRepo.On("AddWatchedFile", "w", f1, false).Return(&database.WatchedFile{}, nil)
	fileRepo.On("AddWatchedFile", "w", f2, false).Return(&database.WatchedFile{}, nil)

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(2), resp.AddedCount)
	assert.Equal(t, int32(0), resp.RemovedCount)
	fileRepo.AssertExpectations(t)
}

func TestSyncWatcher_RemovedFiles(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	// File was in DB but no longer on disk.
	ghost := filepath.Join(dir, "ghost.txt")

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: ghost},
	}, nil)
	fileRepo.On("RemoveWatchedFile", "w", ghost).Return(nil)

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.AddedCount)
	assert.Equal(t, int32(1), resp.RemovedCount)
	assert.Equal(t, []string{ghost}, resp.RemovedFiles)
	fileRepo.AssertExpectations(t)
}

func TestSyncWatcher_FilterApplied(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	// Only .txt files should be accepted; .log should be excluded.
	accepted := filepath.Join(dir, "data.txt")
	rejected := filepath.Join(dir, "debug.log")
	for _, p := range []string{accepted, rejected} {
		require(t, os.WriteFile(p, []byte("x"), 0o644))
	}

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{
		{RuleType: "include", PatternType: "extension", Pattern: ".txt"},
	}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	fileRepo.On("AddWatchedFile", "w", accepted, false).Return(&database.WatchedFile{}, nil)

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(1), resp.AddedCount)
	fileRepo.AssertExpectations(t)
	fileRepo.AssertNotCalled(t, "AddWatchedFile", "w", rejected, true)
}

func TestSyncWatcher_WatcherNotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	svc := newService(fileWatcherRepo, &mocks.MockFileRepository{}, &mocks.MockFilterRepository{})

	fileWatcherRepo.On("GetWatcherByName", "missing").Return(nil, errors.New("not found"))

	_, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "missing"})

	assertCode(t, err, codes.NotFound)
}

func TestSyncWatcher_MissingName(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockFilterRepository{})

	_, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: ""})

	assertCode(t, err, codes.InvalidArgument)
}

func TestSyncWatcher_AlreadyInDB_NotReAdded(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	existing := filepath.Join(dir, "existing.txt")
	require(t, os.WriteFile(existing, []byte("x"), 0o644))

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: existing},
	}, nil)

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.AddedCount)
	assert.Equal(t, int32(0), resp.RemovedCount)
	fileRepo.AssertNotCalled(t, "AddWatchedFile", mock.Anything, mock.Anything, mock.Anything)
}

func TestSyncWatcher_FlushExistingTrue_FilesAddedAsPending(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	f1 := filepath.Join(dir, "existing.txt")
	require(t, os.WriteFile(f1, []byte("x"), 0o644))

	// Watcher has flush_existing=true → files should be added as pending (flushed=false).
	w := newWatcher(1, "w", dir)
	watchedFile := &database.WatchedFile{ID: 1, WatcherName: "w", FilePath: f1, Flushed: true}
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{watchedFile}, nil)
	fileRepo.On("AddWatchedFile", "w", f1, true).Return(&database.WatchedFile{}, nil)
	fileWatcherRepo.On("CreateWatcher", "w", dir).Return(w, nil)

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "w", SourcePath: dir, FlushExisting: true})
	if err != nil {
		t.Fatalf("CreateWatcher error: %v", err)
	}
	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.AddedCount)
	fileRepo.AssertExpectations(t)
}

func TestSyncWatcher_FlushExistingFalse_FilesAddedAsFlushed(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	filterRepo := &mocks.MockFilterRepository{}
	svc := newService(fileWatcherRepo, fileRepo, filterRepo)

	f1 := filepath.Join(dir, "existing.txt")
	require(t, os.WriteFile(f1, []byte("x"), 0o644))

	// Watcher has flush_existing=false (default) → files added as already flushed (flushed=true).
	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	filterRepo.On("GetFiltersForWatcher", "w").Return([]*database.WatcherFilter{}, nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	fileRepo.On("AddWatchedFile", "w", f1, false).Return(&database.WatchedFile{}, nil)

	resp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.Equal(t, int32(1), resp.AddedCount)
	fileRepo.AssertExpectations(t)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	s, ok := status.FromError(err)
	assert.True(t, ok, "expected a gRPC status error")
	assert.Equal(t, want, s.Code())
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// suppress "imported and not used" for mock package
var _ = mock.Anything
