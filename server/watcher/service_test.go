package watcher_test

import (
	"context"
	"errors"
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

func newWatcher(id int64, name, path string, enabled bool) *database.FileWatcher {
	return &database.FileWatcher{
		ID:         id,
		Name:       name,
		SourcePath: path,
		Enabled:    enabled,
		CreatedAt:  fixedAt,
		UpdatedAt:  fixedAt,
	}
}

func newService(fileWatcherRepository *mocks.MockFileWatcherRepository, fileRepository *mocks.MockFileRepository, mgr *mocks.MockWatcherManager) *watcher.WatcherService {
	return watcher.NewManagerService(fileWatcherRepository, fileRepository, mgr, testutil.TestLogger())
}

// ── CreateWatcher ─────────────────────────────────────────────────────────────

func TestCreateWatcher_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	w := newWatcher(1, "my-watcher", "/tmp/src", false)
	fileWatcherRepo.On("CreateWatcher", "my-watcher", "/tmp/src").Return(w, nil)

	resp, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "my-watcher", SourcePath: "/tmp/src"})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.Id)
	assert.Equal(t, "my-watcher", resp.Name)
	assert.Equal(t, "/tmp/src", resp.SourcePath)
	fileWatcherRepo.AssertExpectations(t)
}

func TestCreateWatcher_MissingName(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockWatcherManager{})

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "", SourcePath: "/tmp/src"})

	assertCode(t, err, codes.InvalidArgument)
}

func TestCreateWatcher_MissingSourcePath(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockWatcherManager{})

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "w", SourcePath: ""})

	assertCode(t, err, codes.InvalidArgument)
}

func TestCreateWatcher_DBError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	fileWatcherRepo.On("CreateWatcher", "w", "/tmp").Return(nil, errors.New("db error"))

	_, err := svc.CreateWatcher(ctx, &pb.CreateWatcherRequest{Name: "w", SourcePath: "/tmp"})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── GetWatcherById ────────────────────────────────────────────────────────────

func TestGetWatcherById_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	w := newWatcher(42, "w", "/tmp", false)
	fileWatcherRepo.On("GetWatcherById", int64(42)).Return(w, nil)

	resp, err := svc.GetWatcherById(ctx, &pb.GetWatcherByIdRequest{Id: 42})

	assert.NoError(t, err)
	assert.Equal(t, int64(42), resp.Id)
	fileWatcherRepo.AssertExpectations(t)
}

func TestGetWatcherById_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	fileWatcherRepo.On("GetWatcherById", int64(99)).Return(nil, errors.New("not found"))

	_, err := svc.GetWatcherById(ctx, &pb.GetWatcherByIdRequest{Id: 99})

	assertCode(t, err, codes.NotFound)
	fileWatcherRepo.AssertExpectations(t)
}

// ── GetWatcherByName ──────────────────────────────────────────────────────────

func TestGetWatcherByName_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	w := newWatcher(1, "foo", "/tmp", false)
	fileWatcherRepo.On("GetWatcherByName", "foo").Return(w, nil)

	resp, err := svc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: "foo"})

	assert.NoError(t, err)
	assert.Equal(t, "foo", resp.Name)
	fileWatcherRepo.AssertExpectations(t)
}

func TestGetWatcherByName_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	fileWatcherRepo.On("GetWatcherByName", "missing").Return(nil, errors.New("not found"))

	_, err := svc.GetWatcherByName(ctx, &pb.GetWatcherByNameRequest{Name: "missing"})

	assertCode(t, err, codes.NotFound)
	fileWatcherRepo.AssertExpectations(t)
}

// ── ListWatchers ──────────────────────────────────────────────────────────────

func TestListWatchers_Empty(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	fileWatcherRepo.On("ListWatchers").Return([]*database.FileWatcher{}, nil)

	resp, err := svc.ListWatchers(ctx, &pb.ListWatchersRequest{})

	assert.NoError(t, err)
	assert.Empty(t, resp.Watchers)
	fileWatcherRepo.AssertExpectations(t)
}

func TestListWatchers_Populated(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	watchers := []*database.FileWatcher{
		newWatcher(1, "a", "/tmp/a", false),
		newWatcher(2, "b", "/tmp/b", true),
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
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	fileWatcherRepo.On("ListWatchers").Return(nil, errors.New("db error"))

	_, err := svc.ListWatchers(ctx, &pb.ListWatchersRequest{})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── UpdateWatcher ─────────────────────────────────────────────────────────────

func TestUpdateWatcher_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	w := newWatcher(1, "new-name", "/new/path", false)
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
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	name := "n"
	sourcePath := "/p"

	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 0, Name: &name, SourcePath: &sourcePath})

	assertCode(t, err, codes.InvalidArgument)
}

func TestUpdateWatcher_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	name := "n"
	sourcePath := "/new/path"

	fileWatcherRepo.On("UpdateWatcher", int64(727), &name, &sourcePath).Return(nil, errors.New("not found"))
	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 727, Name: &name, SourcePath: &sourcePath})

	assert.Error(t, err)
}
func TestUpdateWatcher_NullParameter(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	var name *string = nil
	newPath := "/new/path"

	w := newWatcher(1, "old-name", "/new/path", false)
	fileWatcherRepo.On("UpdateWatcher", int64(1), name, &newPath).Return(w, nil)
	_, err := svc.UpdateWatcher(ctx, &pb.UpdateWatcherRequest{Id: 1, SourcePath: &newPath})

	assert.NoError(t, err)
	assert.Equal(t, "old-name", w.Name)
	assert.Equal(t, "/new/path", w.SourcePath)
}

func TestUpdateWatcher_DBError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

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
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	w := newWatcher(1, "to-delete", "/tmp", false)
	key := watcher.WatcherKey{Id: 1, Name: "to-delete"}
	fileWatcherRepo.On("GetWatcherByName", "to-delete").Return(w, nil)
	mgr.On("Stop", key).Return()
	fileWatcherRepo.On("DeleteWatcher", "to-delete").Return(nil)

	resp, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: "to-delete"})

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	fileWatcherRepo.AssertExpectations(t)
	mgr.AssertExpectations(t)
}

func TestDeleteWatcher_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	svc := newService(fileWatcherRepo, fileRepo, &mocks.MockWatcherManager{})

	fileWatcherRepo.On("GetWatcherByName", "ghost").Return(nil, errors.New("not found"))

	_, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: "ghost"})

	assertCode(t, err, codes.NotFound)
	fileWatcherRepo.AssertExpectations(t)
}

func TestDeleteWatcher_DBDeleteError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	w := newWatcher(1, "w", "/tmp", false)
	key := watcher.WatcherKey{Id: 1, Name: "w"}
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	mgr.On("Stop", key).Return()
	fileWatcherRepo.On("DeleteWatcher", "w").Return(errors.New("db error"))

	_, err := svc.DeleteWatcher(ctx, &pb.DeleteWatcherRequest{Name: "w"})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
	mgr.AssertExpectations(t)
}

// ── ToggleWatcher ─────────────────────────────────────────────────────────────

func TestToggleWatcher_Enable_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	w := newWatcher(1, "w", "/tmp/src", true) // enabled after toggle
	key := watcher.WatcherKey{Id: 1, Name: "w"}
	fileWatcherRepo.On("ToggleWatcher", "w").Return(w, nil)
	mgr.On("Start", key, "/tmp/src").Return(nil)

	resp, err := svc.ToggleWatcher(ctx, &pb.ToggleWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.True(t, resp.Enabled)
	fileWatcherRepo.AssertExpectations(t)
	mgr.AssertExpectations(t)
}

func TestToggleWatcher_Disable_OK(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	w := newWatcher(1, "w", "/tmp/src", false) // disabled after toggle
	key := watcher.WatcherKey{Id: 1, Name: "w"}
	fileWatcherRepo.On("ToggleWatcher", "w").Return(w, nil)
	mgr.On("Stop", key).Return()

	resp, err := svc.ToggleWatcher(ctx, &pb.ToggleWatcherRequest{Name: "w"})

	assert.NoError(t, err)
	assert.False(t, resp.Enabled)
	fileWatcherRepo.AssertExpectations(t)
	mgr.AssertExpectations(t)
}

func TestToggleWatcher_Enable_StartFails_Rollback(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	w := newWatcher(1, "w", "/tmp/src", true)
	key := watcher.WatcherKey{Id: 1, Name: "w"}
	fileWatcherRepo.On("ToggleWatcher", "w").Return(w, nil).Once()
	mgr.On("Start", key, "/tmp/src").Return(errors.New("fsnotify error"))
	// rollback: second ToggleWatcher call
	fileWatcherRepo.On("ToggleWatcher", "w").Return(w, nil).Once()

	_, err := svc.ToggleWatcher(ctx, &pb.ToggleWatcherRequest{Name: "w"})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertNumberOfCalls(t, "ToggleWatcher", 2)
	mgr.AssertExpectations(t)
}

func TestToggleWatcher_DBError(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	mgr := &mocks.MockWatcherManager{}
	svc := newService(fileWatcherRepo, fileRepo, mgr)

	fileWatcherRepo.On("ToggleWatcher", "w").Return(nil, errors.New("db error"))

	_, err := svc.ToggleWatcher(ctx, &pb.ToggleWatcherRequest{Name: "w"})

	assertCode(t, err, codes.Internal)
	fileWatcherRepo.AssertExpectations(t)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	s, ok := status.FromError(err)
	assert.True(t, ok, "expected a gRPC status error")
	assert.Equal(t, want, s.Code())
}

// suppress "imported and not used" for mock package
var _ = mock.Anything
