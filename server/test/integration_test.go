//go:build integration

package test

import (
	"os"
	"path/filepath"
	"testing"

	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newDB opens a fresh SQLite database in a temp directory.
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
	srcDir := t.TempDir()

	_, err := db.CreateMachine("test-machine", "hw-id-lifecycle", "10.0.0.1", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)

	w, err := db.CreateWatcher("lifecycle-watcher", srcDir, "test-machine")
	require.NoError(t, err)
	assert.Equal(t, "lifecycle-watcher", w.Name)
	assert.Equal(t, srcDir, w.SourcePath)
	assert.Equal(t, "test-machine", w.MachineName)

	// Update the watcher source path.
	newPath := t.TempDir()
	updated, err := db.UpdateWatcher(w.ID, nil, &newPath)
	require.NoError(t, err)
	assert.Equal(t, newPath, updated.SourcePath)

	// Delete the watcher.
	require.NoError(t, db.DeleteWatcher("lifecycle-watcher"))
	_, err = db.GetWatcherByName("lifecycle-watcher")
	assert.Error(t, err)
}

// ── ListWatchedFiles ──────────────────────────────────────────────────────────

func TestIntegration_ListWatchedFiles(t *testing.T) {
	db := newDB(t)
	srcDir := t.TempDir()

	_, err := db.CreateMachine("test-machine", "hw-id-list", "10.0.0.1", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)

	_, err = db.CreateWatcher("list-watcher", srcDir, "test-machine")
	require.NoError(t, err)

	_, err = db.AddWatchedFile("list-watcher", filepath.Join(srcDir, "pending.txt"), true)
	require.NoError(t, err)
	wf2, err := db.AddWatchedFile("list-watcher", filepath.Join(srcDir, "flushed.txt"), false)
	require.NoError(t, err)

	// Flush wf2 so it no longer appears in ListWatchedFiles.
	require.NoError(t, db.FlushWatchedFiles([]int64{wf2.ID}))

	files, err := db.ListWatchedFiles("list-watcher")
	require.NoError(t, err)
	require.Len(t, files, 2)
}

// ── Cascade delete ────────────────────────────────────────────────────────────

func TestIntegration_WatcherDeleteCascades(t *testing.T) {
	db := newDB(t)
	srcDir := t.TempDir()
	tgtDir := t.TempDir()

	_, err := db.CreateMachine("test-machine", "hw-id-cascade", "10.0.0.1", 22, "ssh-user", "ssh-key")
	require.NoError(t, err)

	_, err = db.CreateWatcher("cascade-watcher", srcDir, "test-machine")
	require.NoError(t, err)

	_, err = db.AddRedirection("cascade-watcher", tgtDir, false)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "drop.txt"), []byte("x"), 0644))
	_, err = db.AddWatchedFile("cascade-watcher", filepath.Join(srcDir, "drop.txt"), false)
	require.NoError(t, err)

	flushes, err := db.ListPendingFlushes("cascade-watcher")
	require.NoError(t, err)
	require.Len(t, flushes, 1)

	// Deleting the watcher must cascade-remove watched_files and file_redirections.
	require.NoError(t, db.DeleteWatcher("cascade-watcher"))

	_, err = db.GetRedirection("cascade-watcher")
	assert.Error(t, err, "redirection should have been cascade-deleted")

	flushes, err = db.ListPendingFlushes("cascade-watcher")
	require.NoError(t, err)
	assert.Empty(t, flushes)
}
