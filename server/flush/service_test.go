package flush_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/flush"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ctx = context.Background()

func newService(repo *mocks.MockFlushRepository) *flush.FlushService {
	return flush.NewFlushService(repo, testutil.TestLogger())
}

func assertCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	s, ok := status.FromError(err)
	assert.True(t, ok, "expected a gRPC status error")
	assert.Equal(t, want, s.Code())
}

// ── FlushWatcher ──────────────────────────────────────────────────────────────

func TestFlushWatcher_EmptyName(t *testing.T) {
	svc := newService(&mocks.MockFlushRepository{})

	_, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: ""})

	assertCode(t, err, codes.InvalidArgument)
}

func TestFlushWatcher_ListPendingFlushes_Error(t *testing.T) {
	repo := &mocks.MockFlushRepository{}
	svc := newService(repo)

	repo.On("ListPendingFlushes", "my-watcher").Return(nil, errors.New("db error"))

	_, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: "my-watcher"})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

func TestFlushWatcher_NoPendingFlushes(t *testing.T) {
	repo := &mocks.MockFlushRepository{}
	svc := newService(repo)

	repo.On("ListPendingFlushes", "my-watcher").Return([]*database.PendingFlush{}, nil)

	resp, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: "my-watcher"})

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	repo.AssertExpectations(t)
}

func TestFlushWatcher_OK(t *testing.T) {
	repo := &mocks.MockFlushRepository{}
	svc := newService(repo)

	// Create a real source file to copy.
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "hello.txt")
	assert.NoError(t, os.WriteFile(srcFile, []byte("hello"), 0o644))

	pendings := []*database.PendingFlush{
		{WatchedFileID: 1, WatcherName: "my-watcher", FilePath: srcDir, FileName: "hello.txt", TargetPath: dstDir},
	}
	repo.On("ListPendingFlushes", "my-watcher").Return(pendings, nil)
	repo.On("FlushWatchedFiles", []int64{1}).Return(nil)

	resp, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: "my-watcher"})

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	// Verify file was copied.
	data, readErr := os.ReadFile(filepath.Join(dstDir, "hello.txt"))
	assert.NoError(t, readErr)
	assert.Equal(t, "hello", string(data))
	repo.AssertExpectations(t)
}

func TestFlushWatcher_FlushWatchedFiles_Error(t *testing.T) {
	repo := &mocks.MockFlushRepository{}
	svc := newService(repo)

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "file.txt")
	assert.NoError(t, os.WriteFile(srcFile, []byte("data"), 0o644))

	pendings := []*database.PendingFlush{
		{WatchedFileID: 2, WatcherName: "my-watcher", FilePath: srcDir, FileName: "file.txt", TargetPath: dstDir},
	}
	repo.On("ListPendingFlushes", "my-watcher").Return(pendings, nil)
	repo.On("FlushWatchedFiles", []int64{2}).Return(errors.New("db error"))

	_, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: "my-watcher"})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

func TestFlushWatcher_CopyFile_SourceMissing(t *testing.T) {
	repo := &mocks.MockFlushRepository{}
	svc := newService(repo)

	dstDir := t.TempDir()
	pendings := []*database.PendingFlush{
		{WatchedFileID: 3, WatcherName: "my-watcher", FilePath: "/nonexistent", FileName: "file.txt", TargetPath: dstDir},
	}
	repo.On("ListPendingFlushes", "my-watcher").Return(pendings, nil)

	_, err := svc.FlushWatcher(ctx, &pb.FlushWatcherRequest{Name: "my-watcher"})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

// suppress "imported and not used" for mock package
var _ = mock.Anything
