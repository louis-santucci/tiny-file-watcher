package watcher_test

// Unit tests for WatcherService.StreamSyncWatcher.
//
// They use the same mock / LocalRemoteFS patterns as service_test.go, and
// additionally inject a MockStreamSyncWatcherServer to capture the sequence of
// SyncWatcherEvent messages sent over the stream.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// allowAllSends configures the stream mock to accept any number of Send calls.
func allowAllSends(stream *mocks.MockStreamSyncWatcherServer) {
	stream.On("Send", mock.Anything).Return(nil)
}

// newStreamServer returns a fresh mock stream server with Send pre-stubbed.
func newStreamServer() *mocks.MockStreamSyncWatcherServer {
	s := &mocks.MockStreamSyncWatcherServer{}
	allowAllSends(s)
	return s
}

// collectLogMessages returns the messages of all LOG events in order.
func collectLogMessages(events []*pb.SyncWatcherEvent) []string {
	var msgs []string
	for _, e := range events {
		if e.Type == pb.SyncWatcherEvent_LOG {
			msgs = append(msgs, e.Message)
		}
	}
	return msgs
}

// findResultEvent returns the first RESULT event, or nil if none.
func findResultEvent(events []*pb.SyncWatcherEvent) *pb.SyncWatcherEvent {
	for _, e := range events {
		if e.Type == pb.SyncWatcherEvent_RESULT {
			return e
		}
	}
	return nil
}

// ── Input validation ──────────────────────────────────────────────────────────

func TestStreamSyncWatcher_MissingName_InvalidArgument(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockMachineRepository{})
	stream := newStreamServer()

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "", Token: testToken}, stream)

	assertCode(t, err, codes.InvalidArgument)
	stream.AssertNotCalled(t, "Send", mock.Anything)
}

func TestStreamSyncWatcher_MissingToken_InvalidArgument(t *testing.T) {
	svc := newService(&mocks.MockFileWatcherRepository{}, &mocks.MockFileRepository{}, &mocks.MockMachineRepository{})
	stream := newStreamServer()

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: ""}, stream)

	assertCode(t, err, codes.InvalidArgument)
	stream.AssertNotCalled(t, "Send", mock.Anything)
}

// ── Authorization ─────────────────────────────────────────────────────────────

func TestStreamSyncWatcher_WatcherNotFound_NotFound(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	svc := newService(fileWatcherRepo, &mocks.MockFileRepository{}, &mocks.MockMachineRepository{})
	stream := newStreamServer()

	fileWatcherRepo.On("GetWatcherByName", "missing").Return(nil, errors.New("not found"))

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "missing", Token: testToken}, stream)

	assertCode(t, err, codes.NotFound)
}

func TestStreamSyncWatcher_UnknownToken_PermissionDenied(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	svc := newService(fileWatcherRepo, &mocks.MockFileRepository{}, machineRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", "/tmp")
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", "bad-token").Return(nil, errors.New("not found"))

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: "bad-token"}, stream)

	assertCode(t, err, codes.PermissionDenied)
}

func TestStreamSyncWatcher_WrongMachine_PermissionDenied(t *testing.T) {
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	svc := newService(fileWatcherRepo, &mocks.MockFileRepository{}, machineRepo)
	stream := newStreamServer()

	w := &database.FileWatcher{ID: 1, MachineName: "machine-A", Name: "w", SourcePath: "/tmp"}
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(&database.Machine{Name: "machine-B"}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assertCode(t, err, codes.PermissionDenied)
}

// ── Event sequence ────────────────────────────────────────────────────────────

func TestStreamSyncWatcher_EmptyDir_SendsExpectedLogSequenceAndResult(t *testing.T) {
	dir := t.TempDir()
	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)

	// Verify LOG messages in order.
	logMsgs := collectLogMessages(stream.Sent)
	assert.Equal(t, []string{
		"loading watched files from database",
		"loading ignore rules",
		"walking source path",
		"computing removed files",
		"saving updates to database",
		"sync complete: 0 file(s) added, 0 file(s) removed",
	}, logMsgs)

	// Verify exactly one RESULT event at the end.
	resultEvt := findResultEvent(stream.Sent)
	assert.NotNil(t, resultEvt)
	assert.NotNil(t, resultEvt.Result)
	assert.Equal(t, int64(0), resultEvt.Result.AddedCount)
	assert.Equal(t, int64(0), resultEvt.Result.RemovedCount)

	// RESULT must be the last event.
	assert.Equal(t, pb.SyncWatcherEvent_RESULT, stream.Sent[len(stream.Sent)-1].Type)
}

func TestStreamSyncWatcher_NewFiles_ResultCarriesAddedCount(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "report.pdf")
	f2 := filepath.Join(dir, "notes.txt")
	RequireNoError(t, os.WriteFile(f1, []byte("x"), 0o644))
	RequireNoError(t, os.WriteFile(f2, []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)

	resultEvt := findResultEvent(stream.Sent)
	assert.NotNil(t, resultEvt)
	assert.Equal(t, int64(2), resultEvt.Result.AddedCount)
	assert.Equal(t, int64(0), resultEvt.Result.RemovedCount)
	assert.ElementsMatch(t, []string{f1, f2}, resultEvt.Result.AddedFiles)
}

func TestStreamSyncWatcher_RemovedFiles_ResultCarriesRemovedCount(t *testing.T) {
	dir := t.TempDir()
	ghost := filepath.Join(dir, "gone.csv")
	// ghost is NOT created on disk — it only lives in the DB.

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{
		{ID: 1, WatcherName: "w", FilePath: ghost},
	}, nil)
	txRepo.On("BulkRemoveWatchedFiles", "w", mock.Anything).Return(nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)

	resultEvt := findResultEvent(stream.Sent)
	assert.NotNil(t, resultEvt)
	assert.Equal(t, int64(0), resultEvt.Result.AddedCount)
	assert.Equal(t, int64(1), resultEvt.Result.RemovedCount)
	assert.Equal(t, []string{ghost}, resultEvt.Result.RemovedFiles)
}

func TestStreamSyncWatcher_SummaryLogMessage_MatchesResult(t *testing.T) {
	dir := t.TempDir()
	RequireNoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644))
	RequireNoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)

	// Find the summary LOG (second-to-last event).
	resultEvt := findResultEvent(stream.Sent)
	assert.NotNil(t, resultEvt)

	// The summary message must reflect the actual counts.
	wantSummary := fmt.Sprintf("sync complete: %d file(s) added, %d file(s) removed",
		resultEvt.Result.AddedCount, resultEvt.Result.RemovedCount)

	logMsgs := collectLogMessages(stream.Sent)
	assert.Equal(t, wantSummary, logMsgs[len(logMsgs)-1])
}

func TestStreamSyncWatcher_ResultIsLastEvent(t *testing.T) {
	dir := t.TempDir()
	RequireNoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)
	assert.NotEmpty(t, stream.Sent)
	assert.Equal(t, pb.SyncWatcherEvent_RESULT, stream.Sent[len(stream.Sent)-1].Type)
}

func TestStreamSyncWatcher_OnlyOneResultEvent(t *testing.T) {
	dir := t.TempDir()

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)

	resultCount := 0
	for _, e := range stream.Sent {
		if e.Type == pb.SyncWatcherEvent_RESULT {
			resultCount++
		}
	}
	assert.Equal(t, 1, resultCount, "exactly one RESULT event must be sent")
}

// ── Stream send error ─────────────────────────────────────────────────────────

func TestStreamSyncWatcher_SendError_OnSummaryLog_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)

	// The stream fails when the summary LOG event is sent (the 6th Send call).
	callCount := 0
	stream := &mocks.MockStreamSyncWatcherServer{}
	stream.On("Send", mock.Anything).Return(nil).Times(5) // first 5 LOG events succeed
	stream.On("Send", mock.MatchedBy(func(evt *pb.SyncWatcherEvent) bool {
		// The 6th Send is the summary LOG
		callCount++
		return evt.Type == pb.SyncWatcherEvent_LOG && callCount > 0
	})).Return(errors.New("stream broken")).Once()
	// Allow subsequent calls (just in case)
	stream.On("Send", mock.Anything).Return(errors.New("stream broken"))

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	// When the summary LOG send fails, StreamSyncWatcher must return an error.
	assert.Error(t, err)
	assertCode(t, err, codes.Internal)
}

// ── Filter applied ────────────────────────────────────────────────────────────

func TestStreamSyncWatcher_FilterApplied_IgnoredFilesExcludedFromResult(t *testing.T) {
	dir := t.TempDir()
	RequireNoError(t, os.WriteFile(filepath.Join(dir, ".tfwignore"), []byte("*.tmp\n"), 0o644))
	kept := filepath.Join(dir, "important.txt")
	ignored := filepath.Join(dir, "scratch.tmp")
	RequireNoError(t, os.WriteFile(kept, []byte("x"), 0o644))
	RequireNoError(t, os.WriteFile(ignored, []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	txRepo.On("BulkAddWatchedFiles", "w", mock.MatchedBy(func(files map[string]string) bool {
		return len(files) == 1
	}), false).Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)
	resultEvt := findResultEvent(stream.Sent)
	assert.NotNil(t, resultEvt)
	assert.Equal(t, int64(1), resultEvt.Result.AddedCount)
	assert.NotContains(t, resultEvt.Result.AddedFiles, ignored)
}

// ── Parity with SyncWatcher ───────────────────────────────────────────────────

// TestStreamSyncWatcher_ResultMatchesSyncWatcher ensures the RESULT event in
// StreamSyncWatcher carries the same data as a direct SyncWatcher call for an
// identical directory state.
func TestStreamSyncWatcher_ResultMatchesSyncWatcher(t *testing.T) {
	dir := t.TempDir()
	RequireNoError(t, os.WriteFile(filepath.Join(dir, "data.csv"), []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)

	w := newWatcher(1, "w", dir)
	machine := newMachineForSync()
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil).Times(2)
	machineRepo.On("GetMachineByToken", testToken).Return(machine, nil).Times(2)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil).Times(2)
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil).Times(2)

	// Non-streaming call.
	syncResp, err := svc.SyncWatcher(ctx, &pb.SyncWatcherRequest{Name: "w", Token: testToken})
	assert.NoError(t, err)

	// Streaming call.
	stream := newStreamServer()
	err = svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)
	assert.NoError(t, err)
	streamResult := findResultEvent(stream.Sent)

	assert.NotNil(t, streamResult)
	assert.Equal(t, syncResp.AddedCount, streamResult.Result.AddedCount)
	assert.Equal(t, syncResp.RemovedCount, streamResult.Result.RemovedCount)
	assert.ElementsMatch(t, syncResp.AddedFiles, streamResult.Result.AddedFiles)
	assert.ElementsMatch(t, syncResp.RemovedFiles, streamResult.Result.RemovedFiles)
}

// ── flush=false forwarded ─────────────────────────────────────────────────────

func TestStreamSyncWatcher_NewFilesAddedAsPending(t *testing.T) {
	dir := t.TempDir()
	RequireNoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	// StreamSyncWatcher always calls Run(false), so BulkAdd receives flushed=false.
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	assert.NoError(t, err)
	txRepo.AssertExpectations(t)
}

// ── watcher.WithSyncJobOptions interaction ────────────────────────────────────

// TestStreamSyncWatcher_SyncJobOpts_Forwarded verifies that SyncJobOptions
// injected via WithSyncJobOptions (e.g. WithRemoteFS) are forwarded to every
// SyncJob created by StreamSyncWatcher.  Without WithRemoteFS the sync would
// attempt an SSH dial; the test passes only if the option is honoured.
func TestStreamSyncWatcher_SyncJobOpts_Forwarded(t *testing.T) {
	dir := t.TempDir()
	RequireNoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644))

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}

	transactor := &mocks.PassthroughTransactor{Repo: txRepo}
	// WithRemoteFS is the only SyncJobOption forwarded; without it the sync
	// would try to SSH-dial and fail.
	svc := watcher.NewManagerService(
		fileWatcherRepo,
		fileRepo,
		machineRepo,
		testutil.TestLogger(),
		transactor,
		watcher.WithSyncJobOptions(watcher.WithRemoteFS(watcher.LocalRemoteFS())),
	)
	stream := newStreamServer()

	w := newWatcher(1, "w", dir)
	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(newMachineForSync(), nil)
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil)
	txRepo.On("BulkAddWatchedFiles", "w", mock.Anything, false).Return([]*database.WatchedFile{}, nil)

	err := svc.StreamSyncWatcher(&pb.SyncWatcherRequest{Name: "w", Token: testToken}, stream)

	// If the SyncJobOption was not forwarded the call would return an SSH error.
	assert.NoError(t, err)
	resultEvt := findResultEvent(stream.Sent)
	assert.NotNil(t, resultEvt)
	assert.Equal(t, int64(1), resultEvt.Result.AddedCount)
}

// ── lock ──────────────────────────────────────────────────────────────────────

// TestStreamSyncWatcher_ConcurrentSync_AlreadyExists verifies that a second
// StreamSyncWatcher call for the same watcher is rejected with AlreadyExists
// while a first streaming sync is still in progress.
//
// The same channel-based blocking pattern used in the SyncWatcher lock test is
// applied here: the first goroutine is held inside SyncJob.Run by a blocking
// ListWatchedFiles mock; the second call must return AlreadyExists immediately.
func TestStreamSyncWatcher_ConcurrentSync_AlreadyExists(t *testing.T) {
	dir := t.TempDir()

	fileWatcherRepo := &mocks.MockFileWatcherRepository{}
	fileRepo := &mocks.MockFileRepository{}
	machineRepo := &mocks.MockMachineRepository{}
	txRepo := &mocks.MockTransactionalFileRepository{}
	svc := newServiceWithLocalFS(fileWatcherRepo, fileRepo, machineRepo, txRepo)

	w := newWatcher(1, "w", dir)
	machine := newMachineForSync()

	release := make(chan struct{})
	entered := make(chan struct{})

	fileWatcherRepo.On("GetWatcherByName", "w").Return(w, nil)
	machineRepo.On("GetMachineByToken", testToken).Return(machine, nil)

	// The first call blocks inside Run; the second call sees the lock is taken.
	fileRepo.On("ListWatchedFiles", "w").
		Run(func(args mock.Arguments) {
			close(entered) // signal: lock is now held
			<-release      // block until released
		}).
		Return([]*database.WatchedFile{}, nil).Once()
	fileRepo.On("ListWatchedFiles", "w").Return([]*database.WatchedFile{}, nil).Maybe()

	req := &pb.SyncWatcherRequest{Name: "w", Token: testToken}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		stream := newStreamServer()
		_ = svc.StreamSyncWatcher(req, stream)
	}()

	// Wait until the first streaming sync has acquired the lock.
	<-entered

	// Second call on the same watcher must be rejected.
	stream2 := newStreamServer()
	err := svc.StreamSyncWatcher(req, stream2)
	assertCode(t, err, codes.AlreadyExists)

	// Unblock the first sync.
	close(release)
	wg.Wait()
}
