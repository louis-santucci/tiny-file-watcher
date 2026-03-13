package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"

	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var key1 = watcher.WatcherKey{Id: 1, Name: "test-watcher"}

func newManager(repo *mocks.MockFileRepository) *watcher.Manager {
	filterRepo := &mocks.MockFilterRepository{}
	filterRepo.On("GetFiltersForWatcher", mock.Anything).Return([]*database.WatcherFilter{}, nil).Maybe()
	return watcher.NewManager(repo, filterRepo, testutil.TestLogger())
}

// ── Start ─────────────────────────────────────────────────────────────────────

func TestManager_Start_OK(t *testing.T) {
	dir := t.TempDir()
	mgr := newManager(&mocks.MockFileRepository{})

	err := mgr.Start(key1, dir)

	assert.NoError(t, err)
	assert.True(t, mgr.IsRunning(key1))
	mgr.Stop(key1)
}

func TestManager_Start_AlreadyRunning_NoOp(t *testing.T) {
	dir := t.TempDir()
	repo := &mocks.MockFileRepository{}
	mgr := newManager(repo)

	assert.NoError(t, mgr.Start(key1, dir))
	// second Start should be a no-op — no error, still running
	assert.NoError(t, mgr.Start(key1, dir))
	assert.True(t, mgr.IsRunning(key1))

	mgr.Stop(key1)
	repo.AssertNotCalled(t, "AddWatchedFile", mock.Anything, mock.Anything)
}

func TestManager_Start_InvalidPath(t *testing.T) {
	mgr := newManager(&mocks.MockFileRepository{})

	err := mgr.Start(key1, "/nonexistent/path/that/does/not/exist")

	assert.Error(t, err)
	assert.False(t, mgr.IsRunning(key1))
}

// ── Stop ──────────────────────────────────────────────────────────────────────

func TestManager_Stop_Running(t *testing.T) {
	dir := t.TempDir()
	mgr := newManager(&mocks.MockFileRepository{})

	assert.NoError(t, mgr.Start(key1, dir))
	mgr.Stop(key1)

	assert.False(t, mgr.IsRunning(key1))
}

func TestManager_Stop_NotRunning_NoOp(t *testing.T) {
	mgr := newManager(&mocks.MockFileRepository{})

	// Should not panic
	mgr.Stop(key1)
	assert.False(t, mgr.IsRunning(key1))
}

// ── IsRunning ─────────────────────────────────────────────────────────────────

func TestManager_IsRunning_False_Before_Start(t *testing.T) {
	mgr := newManager(&mocks.MockFileRepository{})
	assert.False(t, mgr.IsRunning(key1))
}

func TestManager_IsRunning_True_After_Start(t *testing.T) {
	dir := t.TempDir()
	mgr := newManager(&mocks.MockFileRepository{})

	assert.NoError(t, mgr.Start(key1, dir))
	assert.True(t, mgr.IsRunning(key1))

	mgr.Stop(key1)
}

func TestManager_IsRunning_False_After_Stop(t *testing.T) {
	dir := t.TempDir()
	mgr := newManager(&mocks.MockFileRepository{})

	assert.NoError(t, mgr.Start(key1, dir))
	mgr.Stop(key1)
	assert.False(t, mgr.IsRunning(key1))
}

// ── loop: Create event ────────────────────────────────────────────────────────

func TestManager_Loop_CreateEvent_CallsAddWatchedFile(t *testing.T) {
	dir := t.TempDir()
	repo := &mocks.MockFileRepository{}
	mgr := newManager(repo)

	newFile := filepath.Join(dir, "newfile.txt")
	done := make(chan struct{})
	repo.On("AddWatchedFile", key1.Name, newFile, false).
		Return(&database.WatchedFile{}, nil).
		Run(func(_ mock.Arguments) { close(done) })

	assert.NoError(t, mgr.Start(key1, dir))
	defer mgr.Stop(key1)

	f, err := os.Create(newFile)
	assert.NoError(t, err)
	f.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for AddWatchedFile to be called")
	}
	repo.AssertExpectations(t)
}

// ── loop: Remove event ────────────────────────────────────────────────────────

func TestManager_Loop_RemoveEvent_CallsRemoveWatchedFile(t *testing.T) {
	dir := t.TempDir()
	repo := &mocks.MockFileRepository{}
	mgr := newManager(repo)

	existingFile := filepath.Join(dir, "existing.txt")
	f, err := os.Create(existingFile)
	assert.NoError(t, err)
	f.Close()

	done := make(chan struct{})
	repo.On("RemoveWatchedFile", key1.Name, existingFile).
		Return(nil).
		Run(func(_ mock.Arguments) { close(done) })
	// Allow any Create events for the pre-existing file without failing.
	repo.On("AddWatchedFile", key1.Name, mock.Anything).Return(&database.WatchedFile{}, nil).Maybe()

	assert.NoError(t, mgr.Start(key1, dir))
	defer mgr.Stop(key1)

	time.Sleep(50 * time.Millisecond)
	assert.NoError(t, os.Remove(existingFile))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RemoveWatchedFile to be called")
	}
	repo.AssertCalled(t, "RemoveWatchedFile", key1.Name, existingFile)
}

// ── loop: Recursive Create event ─────────────────────────────────────────────

func TestManager_Loop_CreateEvent_InSubfolder_CallsAddWatchedFile(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	assert.NoError(t, os.Mkdir(sub, 0o755))

	repo := &mocks.MockFileRepository{}
	mgr := newManager(repo)

	newFile := filepath.Join(sub, "nested.txt")
	done := make(chan struct{})
	repo.On("AddWatchedFile", key1.Name, newFile, false).
		Return(&database.WatchedFile{}, nil).
		Run(func(_ mock.Arguments) { close(done) })

	assert.NoError(t, mgr.Start(key1, dir))
	defer mgr.Stop(key1)

	f, err := os.Create(newFile)
	assert.NoError(t, err)
	f.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for AddWatchedFile to be called for nested file")
	}
	repo.AssertExpectations(t)
}

func TestManager_Loop_CreateEvent_Directory_DoesNotCallAddWatchedFile(t *testing.T) {
	dir := t.TempDir()
	repo := &mocks.MockFileRepository{}
	mgr := newManager(repo)

	assert.NoError(t, mgr.Start(key1, dir))
	defer mgr.Stop(key1)

	assert.NoError(t, os.Mkdir(filepath.Join(dir, "newdir"), 0o755))

	// Give time for any (unwanted) event to be processed.
	time.Sleep(200 * time.Millisecond)

	repo.AssertNotCalled(t, "AddWatchedFile", mock.Anything, mock.Anything, mock.Anything)
}
