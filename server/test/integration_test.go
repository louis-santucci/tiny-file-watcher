//go:build integration

package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/test/testutil"
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDB opens a fresh SQLite database in a temp directory.
// The file URI pragmas ensure:
//   - foreign_keys: enabled for every connection in the pool (not just the first)
//   - journal_mode=WAL: allow concurrent readers while the Manager goroutine writes
//   - busy_timeout: retry briefly on lock contention instead of returning SQLITE_BUSY
func newDB(t *testing.T) *database.DB {
	t.Helper()
	path := "file:" + filepath.Join(t.TempDir(), "test.db") +
		"?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := database.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// ── Watcher lifecycle ─────────────────────────────────────────────────────────

func TestIntegration_WatcherLifecycle(t *testing.T) {
	db := newDB(t)
	mgr := watcher.NewManager(db, testutil.TestLogger())
	srcDir := t.TempDir()

	w, err := db.CreateWatcher("lifecycle-watcher", srcDir)
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-watcher", w.Name)
	assert.False(t, w.Enabled)

	key := watcher.WatcherKey{Id: w.ID, Name: w.Name}
	assert.False(t, mgr.IsRunning(key))

	// Enable: toggle via DB + start manager
	toggled, err := db.ToggleWatcher("lifecycle-watcher")
	require.NoError(t, err)
	assert.True(t, toggled.Enabled)
	require.NoError(t, mgr.Start(key, srcDir))
	assert.True(t, mgr.IsRunning(key))

	// Disable: toggle via DB + stop manager
	toggled, err = db.ToggleWatcher("lifecycle-watcher")
	require.NoError(t, err)
	assert.False(t, toggled.Enabled)
	mgr.Stop(key)
	assert.False(t, mgr.IsRunning(key))
}

// ── File detection ────────────────────────────────────────────────────────────

func TestIntegration_FileDetection(t *testing.T) {
	db := newDB(t)
	mgr := watcher.NewManager(db, testutil.TestLogger())
	srcDir := t.TempDir()
	tgtDir := t.TempDir()

	w, err := db.CreateWatcher("detect-watcher", srcDir)
	require.NoError(t, err)

	// Add a redirection upfront so pending_file_flushes view returns rows.
	_, err = db.AddRedirection("detect-watcher", tgtDir, false)
	require.NoError(t, err)

	key := watcher.WatcherKey{Id: w.ID, Name: w.Name}
	require.NoError(t, mgr.Start(key, srcDir))
	defer mgr.Stop(key)

	// Drop a file into the watched directory.
	f, err := os.Create(filepath.Join(srcDir, "hello.txt"))
	require.NoError(t, err)
	f.Close()

	// Wait until the manager has recorded the file.
	require.Eventually(t, func() bool {
		flushes, _ := db.ListPendingFlushes("detect-watcher")
		return len(flushes) > 0
	}, 2*time.Second, 50*time.Millisecond, "timed out waiting for file detection")

	flushes, err := db.ListPendingFlushes("detect-watcher")
	require.NoError(t, err)
	require.Len(t, flushes, 1)
	assert.Equal(t, "hello.txt", flushes[0].FileName)
}

// ── Cascade delete ────────────────────────────────────────────────────────────

func TestIntegration_WatcherDeleteCascades(t *testing.T) {
	db := newDB(t)
	mgr := watcher.NewManager(db, testutil.TestLogger())
	srcDir := t.TempDir()
	tgtDir := t.TempDir()

	w, err := db.CreateWatcher("cascade-watcher", srcDir)
	require.NoError(t, err)

	_, err = db.AddRedirection("cascade-watcher", tgtDir, false)
	require.NoError(t, err)

	key := watcher.WatcherKey{Id: w.ID, Name: w.Name}
	require.NoError(t, mgr.Start(key, srcDir))

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "drop.txt"), []byte("x"), 0644))

	require.Eventually(t, func() bool {
		flushes, _ := db.ListPendingFlushes("cascade-watcher")
		return len(flushes) > 0
	}, 2*time.Second, 50*time.Millisecond, "timed out waiting for file detection")

	mgr.Stop(key)

	// Give the manager goroutine a moment to drain before deleting the watcher,
	// so that any in-flight AddWatchedFile call completes (and will hit FK error)
	// rather than racing with the cascade delete.
	time.Sleep(100 * time.Millisecond)

	// Deleting the watcher must cascade-remove watched_files and file_redirections.
	require.NoError(t, db.DeleteWatcher("cascade-watcher"))

	_, err = db.GetRedirection("cascade-watcher")
	assert.Error(t, err, "redirection should have been cascade-deleted")

	flushes, err := db.ListPendingFlushes("cascade-watcher")
	require.NoError(t, err)
	assert.Empty(t, flushes)
}

// ── Nested file detection ─────────────────────────────────────────────────────

func TestIntegration_NestedFileDetection(t *testing.T) {
	db := newDB(t)
	mgr := watcher.NewManager(db, testutil.TestLogger())
	srcDir := t.TempDir()
	tgtDir := t.TempDir()

	w, err := db.CreateWatcher("nested-watcher", srcDir)
	require.NoError(t, err)

	_, err = db.AddRedirection("nested-watcher", tgtDir, false)
	require.NoError(t, err)

	key := watcher.WatcherKey{Id: w.ID, Name: w.Name}
	require.NoError(t, mgr.Start(key, srcDir))
	defer mgr.Stop(key)

	// Create a subfolder then drop a file inside it.
	// A short pause lets the manager process the directory Create event
	// and register the new subdirectory with fsnotify before the file lands.
	subDir := filepath.Join(srcDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0o755))
	time.Sleep(100 * time.Millisecond)

	expectedPath := filepath.Join(subDir, "nested.txt")
	require.NoError(t, os.WriteFile(expectedPath, []byte("hello"), 0o644))

	// Wait until the manager records the nested file.
	require.Eventually(t, func() bool {
		flushes, _ := db.ListPendingFlushes("nested-watcher")
		return len(flushes) > 0
	}, 2*time.Second, 50*time.Millisecond, "timed out waiting for nested file detection")

	flushes, err := db.ListPendingFlushes("nested-watcher")
	require.NoError(t, err)
	require.Len(t, flushes, 1)
	assert.Equal(t, "nested-watcher", flushes[0].WatcherName)
	assert.Equal(t, "nested.txt", flushes[0].FileName)
	assert.Equal(t, expectedPath, flushes[0].FilePath)
	assert.Equal(t, tgtDir, flushes[0].TargetPath)

	// Remove the file and verify the manager removes the pending flush.
	require.NoError(t, os.Remove(expectedPath))

	require.Eventually(t, func() bool {
		flushes, _ := db.ListPendingFlushes("nested-watcher")
		return len(flushes) == 0
	}, 2*time.Second, 50*time.Millisecond, "timed out waiting for pending flush removal")
}
