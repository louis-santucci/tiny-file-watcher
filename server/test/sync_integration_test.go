//go:build integration

package test

// Integration tests for WatcherService.SyncWatcher and
// WatcherService.StreamSyncWatcher.
//
// These tests exercise the full stack end-to-end:
//   - real SQLite database (opened in a temp dir)
//   - real local filesystem (via watcher.LocalRemoteFS)
//   - the complete WatcherService gRPC handler
//
// Run with: go test -tags=integration ./server/test/...

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

const integrationToken = "integ-token-1234"

var integrationSSHConfig = &config.SSHConfig{PrivateKeysPath: "/tmp/keys", KnownHostsPath: "/tmp/known_hosts"}

// newSyncService creates a WatcherService backed by a real SQLite DB.
func newSyncService(t *testing.T) (*watcher.WatcherService, *database.DB) {
	t.Helper()
	db := newDB(t)
	svc := watcher.NewManagerService(
		db, db, db,
		testutil.TestLogger(),
		integrationSSHConfig,
		db,
		watcher.WithSyncJobOptions(watcher.WithRemoteFS(watcher.LocalRemoteFS())),
	)
	return svc, db
}

// seedMachineAndWatcher creates the necessary DB rows and returns the watcher.
func seedMachineAndWatcher(t *testing.T, db *database.DB, srcDir string) {
	t.Helper()
	_, err := db.CreateMachine("integ-machine", integrationToken, "127.0.0.1", 22, "user", "key")
	require.NoError(t, err)
	_, err = db.CreateWatcher("integ-watcher", srcDir, "integ-machine")
	require.NoError(t, err)
}

// writeTestFile creates a file with content "x" at the given full path.
func writeTestFile(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))
}

// syncReq is a convenience for building SyncWatcherRequest.
func syncReq() *pb.SyncWatcherRequest {
	return &pb.SyncWatcherRequest{Name: "integ-watcher", Token: integrationToken}
}

// ── SyncWatcher integration ───────────────────────────────────────────────────

func TestIntegration_SyncWatcher_NewFilesAddedToDB(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	writeTestFile(t, filepath.Join(srcDir, "alpha.txt"))
	writeTestFile(t, filepath.Join(srcDir, "beta.txt"))

	resp, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.AddedCount)
	assert.Equal(t, int64(0), resp.RemovedCount)
	assert.Len(t, resp.AddedFiles, 2)

	// Verify the files are actually in the DB.
	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestIntegration_SyncWatcher_Idempotent(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	writeTestFile(t, filepath.Join(srcDir, "file.txt"))

	// First sync — adds 1 file.
	resp1, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp1.AddedCount)

	// Second sync — same directory, nothing changed.
	resp2, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp2.AddedCount)
	assert.Equal(t, int64(0), resp2.RemovedCount)

	// Only 1 row in the DB after two syncs.
	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestIntegration_SyncWatcher_DeletedFilesRemovedFromDB(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	filePath := filepath.Join(srcDir, "to-delete.txt")
	writeTestFile(t, filePath)

	// First sync — adds the file.
	resp1, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	require.Equal(t, int64(1), resp1.AddedCount)

	// Delete the file from disk.
	require.NoError(t, os.Remove(filePath))

	// Second sync — file is gone from disk, should be removed from DB.
	resp2, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp2.AddedCount)
	assert.Equal(t, int64(1), resp2.RemovedCount)
	assert.Equal(t, []string{filePath}, resp2.RemovedFiles)

	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestIntegration_SyncWatcher_NewFilesAreNotFlushed(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	writeTestFile(t, filepath.Join(srcDir, "pending.txt"))

	_, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)

	// SyncWatcher calls Run(flush=false) — new files must have Flushed=false.
	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.False(t, files[0].Flushed, "SyncWatcher must add files as pending (flushed=false)")
}

func TestIntegration_SyncWatcher_SubdirectoryFilesDetected(t *testing.T) {
	srcDir := t.TempDir()
	subDir := filepath.Join(srcDir, "sub")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	writeTestFile(t, filepath.Join(srcDir, "root.txt"))
	writeTestFile(t, filepath.Join(subDir, "nested.txt"))

	resp, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.AddedCount)

	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestIntegration_SyncWatcher_IgnoreRulesRespected(t *testing.T) {
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, ".tfwignore"), []byte("*.log\n"), 0o644))

	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	writeTestFile(t, filepath.Join(srcDir, "data.csv"))
	writeTestFile(t, filepath.Join(srcDir, "debug.log"))

	resp, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.AddedCount, "only data.csv should be added")

	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].FilePath, "data.csv")
}

func TestIntegration_SyncWatcher_AddAndDeleteConcurrently(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	// Start with two files.
	file1 := filepath.Join(srcDir, "keep.txt")
	file2 := filepath.Join(srcDir, "remove.txt")
	writeTestFile(t, file1)
	writeTestFile(t, file2)

	resp1, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	require.Equal(t, int64(2), resp1.AddedCount)

	// Remove file2 and add file3.
	require.NoError(t, os.Remove(file2))
	file3 := filepath.Join(srcDir, "new.txt")
	writeTestFile(t, file3)

	resp2, err := svc.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp2.AddedCount)
	assert.Equal(t, int64(1), resp2.RemovedCount)
	assert.Equal(t, []string{file3}, resp2.AddedFiles)
	assert.Equal(t, []string{file2}, resp2.RemovedFiles)

	// DB should now have file1 and file3 only.
	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	require.Len(t, files, 2)
	var paths []string
	for _, f := range files {
		paths = append(paths, f.FilePath)
	}
	assert.ElementsMatch(t, []string{file1, file3}, paths)
}

// ── StreamSyncWatcher integration ────────────────────────────────────────────

func TestIntegration_StreamSyncWatcher_EmptyDir_LogsAndResult(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil)

	err := svc.StreamSyncWatcher(syncReq(), stream)
	require.NoError(t, err)

	// Expect 6 LOG events + 1 RESULT event.
	assert.Len(t, stream.Sent, 7)
	assert.Equal(t, pb.SyncWatcherEvent_RESULT, stream.Sent[len(stream.Sent)-1].Type)
}

func TestIntegration_StreamSyncWatcher_NewFilesInResult(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	f1 := filepath.Join(srcDir, "a.dat")
	f2 := filepath.Join(srcDir, "b.dat")
	writeTestFile(t, f1)
	writeTestFile(t, f2)

	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil)

	err := svc.StreamSyncWatcher(syncReq(), stream)
	require.NoError(t, err)

	// Find the RESULT event.
	var resultEvt *pb.SyncWatcherEvent
	for _, e := range stream.Sent {
		if e.Type == pb.SyncWatcherEvent_RESULT {
			resultEvt = e
		}
	}
	require.NotNil(t, resultEvt)
	assert.Equal(t, int64(2), resultEvt.Result.AddedCount)
	assert.ElementsMatch(t, []string{f1, f2}, resultEvt.Result.AddedFiles)

	// Verify DB state matches.
	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestIntegration_StreamSyncWatcher_RemovedFilesInResult(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	filePath := filepath.Join(srcDir, "ephemeral.txt")
	writeTestFile(t, filePath)

	// First sync — adds the file.
	stream1 := &mocks.MockStreamSyncWatcherServer{}
	stream1.On("Send", mock.Anything).Return(nil)
	require.NoError(t, svc.StreamSyncWatcher(syncReq(), stream1))

	// Remove the file from disk.
	require.NoError(t, os.Remove(filePath))

	// Second sync — file must appear in RemovedFiles.
	stream2 := &mocks.MockStreamSyncWatcherServer{}
	stream2.On("Send", mock.Anything).Return(nil)
	require.NoError(t, svc.StreamSyncWatcher(syncReq(), stream2))

	var resultEvt *pb.SyncWatcherEvent
	for _, e := range stream2.Sent {
		if e.Type == pb.SyncWatcherEvent_RESULT {
			resultEvt = e
		}
	}
	require.NotNil(t, resultEvt)
	assert.Equal(t, int64(1), resultEvt.Result.RemovedCount)
	assert.Equal(t, []string{filePath}, resultEvt.Result.RemovedFiles)

	// DB should be empty after removal.
	files, err := db.ListWatchedFiles("integ-watcher")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestIntegration_StreamSyncWatcher_LogMessages_Ordered(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil)

	require.NoError(t, svc.StreamSyncWatcher(syncReq(), stream))

	var logMsgs []string
	for _, e := range stream.Sent {
		if e.Type == pb.SyncWatcherEvent_LOG {
			logMsgs = append(logMsgs, e.Message)
		}
	}
	assert.Equal(t, []string{
		"loading watched files from database",
		"loading ignore rules",
		"walking source path",
		"computing removed files",
		"saving updates to database",
		"sync complete: 0 file(s) added, 0 file(s) removed",
	}, logMsgs)
}

func TestIntegration_StreamSyncWatcher_SummaryMatchesResult(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	writeTestFile(t, filepath.Join(srcDir, "one.txt"))
	writeTestFile(t, filepath.Join(srcDir, "two.txt"))

	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil)
	require.NoError(t, svc.StreamSyncWatcher(syncReq(), stream))

	// Extract summary LOG and RESULT.
	var summaryMsg string
	var resultEvt *pb.SyncWatcherEvent
	for _, e := range stream.Sent {
		if e.Type == pb.SyncWatcherEvent_LOG {
			summaryMsg = e.Message // last LOG wins
		} else if e.Type == pb.SyncWatcherEvent_RESULT {
			resultEvt = e
		}
	}
	require.NotNil(t, resultEvt)
	wantSummary := fmt.Sprintf("sync complete: %d file(s) added, %d file(s) removed",
		resultEvt.Result.AddedCount, resultEvt.Result.RemovedCount)
	assert.Equal(t, wantSummary, summaryMsg)
}

func TestIntegration_StreamSyncWatcher_ResultIsLastEvent(t *testing.T) {
	srcDir := t.TempDir()
	svc, db := newSyncService(t)
	seedMachineAndWatcher(t, db, srcDir)

	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil)
	require.NoError(t, svc.StreamSyncWatcher(syncReq(), stream))

	require.NotEmpty(t, stream.Sent)
	assert.Equal(t, pb.SyncWatcherEvent_RESULT, stream.Sent[len(stream.Sent)-1].Type)
}

// ── Parity: SyncWatcher vs StreamSyncWatcher ─────────────────────────────────

func TestIntegration_Parity_SyncAndStreamProduceSameResult(t *testing.T) {
	srcDir := t.TempDir()
	writeTestFile(t, filepath.Join(srcDir, "x.txt"))
	writeTestFile(t, filepath.Join(srcDir, "y.txt"))

	// Use separate DB instances so the two calls start from the same state.
	svc1, db1 := newSyncService(t)
	_, err := db1.CreateMachine("m", integrationToken, "127.0.0.1", 22, "u", "k")
	require.NoError(t, err)
	_, err = db1.CreateWatcher("integ-watcher", srcDir, "m")
	require.NoError(t, err)

	svc2, db2 := newSyncService(t)
	_, err = db2.CreateMachine("m", integrationToken, "127.0.0.1", 22, "u", "k")
	require.NoError(t, err)
	_, err = db2.CreateWatcher("integ-watcher", srcDir, "m")
	require.NoError(t, err)

	// Non-streaming call.
	syncResp, err := svc1.SyncWatcher(context.Background(), syncReq())
	require.NoError(t, err)

	// Streaming call.
	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil)
	require.NoError(t, svc2.StreamSyncWatcher(syncReq(), stream))

	var streamResult *pb.SyncWatcherResponse
	for _, e := range stream.Sent {
		if e.Type == pb.SyncWatcherEvent_RESULT {
			streamResult = e.Result
		}
	}
	require.NotNil(t, streamResult)

	assert.Equal(t, syncResp.AddedCount, streamResult.AddedCount)
	assert.Equal(t, syncResp.RemovedCount, streamResult.RemovedCount)
	assert.ElementsMatch(t, syncResp.AddedFiles, streamResult.AddedFiles)
	assert.ElementsMatch(t, syncResp.RemovedFiles, streamResult.RemovedFiles)
}
