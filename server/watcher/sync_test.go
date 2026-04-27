package watcher_test

// Unit tests for the SyncJob core sync algorithm (watcher.SyncJob.Run).
//
// These tests bypass SSH entirely by injecting LocalRemoteFS() and drive the
// full walk→diff→persist pipeline against real temp directories and testify
// mocks for the database layer.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"tiny-file-watcher/internal"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newSyncJob(
	dir string,
	fileRepo database.FileRepository,
	fileWatcherRepo database.FileWatcherRepository,
	transactor database.Transactor,
	opts ...watcher.SyncJobOption,
) *watcher.SyncJob {
	w := &database.FileWatcher{ID: 1, Name: "w", SourcePath: dir, MachineName: "test-machine"}
	machine := &database.Machine{ID: 1, Name: "test-machine"}
	allOpts := append([]watcher.SyncJobOption{watcher.WithRemoteFS(watcher.LocalRemoteFS())}, opts...)
	return watcher.NewSyncJob(
		testutil.TestLogger(),
		w,
		machine,
		fileRepo,
		fileWatcherRepo,
		transactor,
		allOpts...,
	)
}

// writeFile creates an empty file at the given full path.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// sortedStrings sorts a string slice in-place and returns it (for assertion readability).
func sortedStrings(s []string) []string {
	sort.Strings(s)
	return s
}

// ── SyncJob.Run: basic add ────────────────────────────────────────────────────

func TestSyncJob_NewFiles_AreAdded(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	writeFile(t, f1)
	writeFile(t, f2)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files *internal.Set[string]) bool {
		return files.Size() == 2
	}), false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(2), result.AddedCount)
	assert.Equal(t, int32(0), result.RemovedCount)
	assert.ElementsMatch(t, []string{f1, f2}, result.AddedFiles.Items())
	txRepo.AssertExpectations(t)
}

func TestSyncJob_FlushTrue_FilesInsertedAsFlushed(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"))

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	// flush=true is forwarded to BulkAddWatchedFiles
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, true).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(true)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), result.AddedCount)
	txRepo.AssertExpectations(t)
}

func TestSyncJob_FlushFalse_FilesInsertedAsPending(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"))

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	// flush=false is forwarded to BulkAddWatchedFiles
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), result.AddedCount)
	txRepo.AssertExpectations(t)
}

// ── SyncJob.Run: no-op when files already tracked ────────────────────────────

func TestSyncJob_ExistingFiles_NotReAdded(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "existing.txt")
	writeFile(t, f)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: f},
	}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(0), result.AddedCount)
	assert.Equal(t, int32(0), result.RemovedCount)
	txRepo.AssertNotCalled(t, "BulkAddWatchedFiles", mock.Anything, mock.Anything, mock.Anything)
	txRepo.AssertNotCalled(t, "BulkRemoveWatchedFiles", mock.Anything, mock.Anything)
}

// ── SyncJob.Run: removals ─────────────────────────────────────────────────────

func TestSyncJob_DeletedFiles_AreRemoved(t *testing.T) {
	dir := t.TempDir()
	// ghost.txt was in the DB but is not on disk
	ghost := filepath.Join(dir, "ghost.txt")

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: ghost},
	}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkRemoveWatchedFiles", "w", mock.MatchedBy(func(paths []string) bool {
		return len(paths) == 1 && paths[0] == ghost
	})).Return(nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(0), result.AddedCount)
	assert.Equal(t, int32(1), result.RemovedCount)
	assert.Equal(t, []string{ghost}, result.RemovedFiles)
	txRepo.AssertExpectations(t)
}

func TestSyncJob_AddAndRemove_Simultaneously(t *testing.T) {
	dir := t.TempDir()
	// kept.txt is both on disk and in the DB — should not be added or removed.
	kept := filepath.Join(dir, "kept.txt")
	// new.txt is on disk but not in the DB — should be added.
	newFile := filepath.Join(dir, "new.txt")
	// gone.txt is in the DB but not on disk — should be removed.
	gone := filepath.Join(dir, "gone.txt")

	writeFile(t, kept)
	writeFile(t, newFile)
	// gone.txt intentionally NOT created on disk

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: kept},
		{ID: 2, WatcherName: "w", FilePath: gone},
	}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files *internal.Set[string]) bool {
		if files.Size() != 1 {
			return false
		}
		for _, v := range files.Items() {
			if v == newFile {
				return true
			}
		}
		return false
	}), false).Return([]*database.WatchedFile{}, nil)
	txRepo.On("BulkRemoveWatchedFiles", "w", mock.MatchedBy(func(paths []string) bool {
		return len(paths) == 1 && paths[0] == gone
	})).Return(nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), result.AddedCount)
	assert.Equal(t, int32(1), result.RemovedCount)
	assert.Equal(t, []string{newFile}, result.AddedFiles.Items())
	assert.Equal(t, []string{gone}, result.RemovedFiles)
	txRepo.AssertExpectations(t)
}

// ── SyncJob.Run: empty directory ──────────────────────────────────────────────

func TestSyncJob_EmptyDirectory_NoChanges(t *testing.T) {
	dir := t.TempDir()

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(0), result.AddedCount)
	assert.Equal(t, int32(0), result.RemovedCount)
	txRepo.AssertNotCalled(t, "BulkAddWatchedFiles", mock.Anything, mock.Anything, mock.Anything)
	txRepo.AssertNotCalled(t, "BulkRemoveWatchedFiles", mock.Anything, mock.Anything)
}

// ── SyncJob.Run: subdirectory traversal ──────────────────────────────────────

func TestSyncJob_SubdirectoryFiles_AreDiscovered(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(subDir, "deep.txt")
	writeFile(t, deep)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files *internal.Set[string]) bool {
		for _, v := range files.Items() {
			if v == deep {
				return true
			}
		}
		return false
	}), false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), result.AddedCount)
	assert.Contains(t, result.AddedFiles.Items(), deep)
	txRepo.AssertExpectations(t)
}

func TestSyncJob_NestedSubdirectories_AllFilesDiscovered(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	ab := filepath.Join(a, "b")
	if err := os.MkdirAll(ab, 0o755); err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(dir, "root.txt")
	f2 := filepath.Join(a, "mid.txt")
	f3 := filepath.Join(ab, "leaf.txt")
	writeFile(t, f1)
	writeFile(t, f2)
	writeFile(t, f3)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files *internal.Set[string]) bool {
		return files.Size() == 3
	}), false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(3), result.AddedCount)
	assert.ElementsMatch(t, []string{f1, f2, f3}, sortedStrings(result.AddedFiles.Items()))
	txRepo.AssertExpectations(t)
}

// ── SyncJob.Run: .tfwignore filtering ────────────────────────────────────────

func TestSyncJob_IgnoredFiles_NotAdded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tfwignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	accepted := filepath.Join(dir, "data.txt")
	ignored := filepath.Join(dir, "debug.log")
	writeFile(t, accepted)
	writeFile(t, ignored)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files *internal.Set[string]) bool {
		if files.Size() != 1 {
			return false
		}
		for _, v := range files.Items() {
			if v == accepted {
				return true
			}
		}
		return false
	}), false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), result.AddedCount)
	assert.NotContains(t, result.AddedFiles.Items(), ignored)
	txRepo.AssertExpectations(t)
}

func TestSyncJob_TfwignoreFileItself_NotTracked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tfwignore"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(dir, "keep.txt")
	writeFile(t, f)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files *internal.Set[string]) bool {
		for _, name := range files.Items() {
			if filepath.Base(name) == ".tfwignore" {
				return false
			}
		}
		return true
	}), false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, int32(1), result.AddedCount)
	for _, f := range result.AddedFiles.Items() {
		assert.NotEqual(t, ".tfwignore", filepath.Base(f))
	}
	txRepo.AssertExpectations(t)
}

// ── SyncJob.Run: log callback ─────────────────────────────────────────────────

func TestSyncJob_LogCallback_ReceivesExpectedMessages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"))

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	var logMessages []string
	logCallback := watcher.WithLogCallback(func(msg string) {
		logMessages = append(logMessages, msg)
	})

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo}, logCallback)
	_, err := job.Run(false)

	assert.NoError(t, err)
	assert.Equal(t, []string{
		"loading watched files from database",
		"loading ignore rules",
		"walking source path",
		"computing removed files",
		"saving updates to database",
	}, logMessages)
}

func TestSyncJob_NoLogCallback_DoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"))

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	// No WithLogCallback option — onLog is nil; must not panic.
	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	assert.NotPanics(t, func() {
		_, _ = job.Run(false)
	})
}

// ── SyncJob.Run: DB errors ────────────────────────────────────────────────────

func TestSyncJob_ListWatchedFiles_DBError_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return(nil, errors.New("db boom"))

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.NoopTransactor{})
	_, err := job.Run(false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "db boom")
}

func TestSyncJob_BulkAdd_DBError_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "file.txt"))

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return(nil, errors.New("insert failed"))

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	_, err := job.Run(false)

	assert.Error(t, err)
	txRepo.AssertExpectations(t)
}

func TestSyncJob_BulkRemove_DBError_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	// ghost.txt is in DB but not on disk
	ghost := filepath.Join(dir, "ghost.txt")

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: ghost},
	}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkRemoveWatchedFiles", "w", mock.Anything).Return(errors.New("delete failed"))

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	_, err := job.Run(false)

	assert.Error(t, err)
	txRepo.AssertExpectations(t)
}

// ── SyncJob.Run: idempotency ──────────────────────────────────────────────────

func TestSyncJob_RunTwice_Idempotent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	writeFile(t, f)

	// First run: file is new
	fileRepo1 := &mocks.MockFileRepository{}
	fileRepo1.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	txRepo1 := &mocks.MockTransactionalFileRepository{}
	txRepo1.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	job1 := newSyncJob(dir, fileRepo1, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo1})
	result1, err := job1.Run(false)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), result1.AddedCount)

	// Second run: file is already tracked
	fileRepo2 := &mocks.MockFileRepository{}
	fileRepo2.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: f},
	}, nil)
	txRepo2 := &mocks.MockTransactionalFileRepository{}

	job2 := newSyncJob(dir, fileRepo2, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo2})
	result2, err := job2.Run(false)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), result2.AddedCount)
	assert.Equal(t, int32(0), result2.RemovedCount)
	txRepo2.AssertNotCalled(t, "BulkAddWatchedFiles", mock.Anything, mock.Anything, mock.Anything)
}

// ── SyncJob.Run: result fields ────────────────────────────────────────────────

func TestSyncJob_Result_AddedFilesContainFullPaths(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "report.csv")
	writeFile(t, f)

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Len(t, result.AddedFiles.Items(), 1)
	assert.Equal(t, f, result.AddedFiles.Items()[0])
}

func TestSyncJob_Result_RemovedFilesContainFullPaths(t *testing.T) {
	dir := t.TempDir()
	ghost := filepath.Join(dir, "gone.log")

	fileRepo := &mocks.MockFileRepository{}
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: ghost},
	}, nil)

	txRepo := &mocks.MockTransactionalFileRepository{}
	txRepo.On("BulkRemoveWatchedFiles", "w", mock.Anything).Return(nil)

	job := newSyncJob(dir, fileRepo, &mocks.MockFileWatcherRepository{}, &mocks.PassthroughTransactor{Repo: txRepo})
	result, err := job.Run(false)

	assert.NoError(t, err)
	assert.Len(t, result.RemovedFiles, 1)
	assert.Equal(t, ghost, result.RemovedFiles[0])
}
