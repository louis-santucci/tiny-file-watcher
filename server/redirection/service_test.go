package redirection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "tiny-file-watcher/gen/grpc"
	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/redirection"
	"tiny-file-watcher/server/test/mocks"
	"tiny-file-watcher/server/test/testutil"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ctx     = context.Background()
	fixedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
)

func newRedirection(watcherName, targetPath string, autoFlush bool) *database.FileRedirection {
	return &database.FileRedirection{
		WatcherName: watcherName,
		TargetPath:  targetPath,
		AutoFlush:   autoFlush,
		CreatedAt:   fixedAt,
		UpdatedAt:   fixedAt,
	}
}

func newService(repo *mocks.MockRedirectionRepository) *redirection.RedirectionService {
	return redirection.NewRedirectionService(
		&mocks.MockFileWatcherRepository{},
		&mocks.MockFileRepository{},
		repo,
		testutil.TestLogger(),
	)
}

func assertCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	assert.True(t, ok, "expected gRPC status error")
	assert.Equal(t, code, st.Code())
}

// ── AddRedirection ────────────────────────────────────────────────────────────

func TestAddRedirection_OK(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	r := newRedirection("my-watcher", "/tmp/out", true)
	repo.On("AddRedirection", "my-watcher", "/tmp/out", true).Return(r, nil)

	resp, err := svc.CreateFileRedirection(ctx, &pb.CreateFileRedirectionRequest{
		WatcherName: "my-watcher",
		TargetPath:  "/tmp/out",
		AutoFlush:   true,
	})

	assert.NoError(t, err)
	assert.Equal(t, "my-watcher", resp.WatcherName)
	assert.Equal(t, "/tmp/out", resp.TargetPath)
	assert.True(t, resp.AutoFlush)
	repo.AssertExpectations(t)
}

func TestAddRedirection_MissingWatcherName(t *testing.T) {
	svc := newService(&mocks.MockRedirectionRepository{})

	_, err := svc.CreateFileRedirection(ctx, &pb.CreateFileRedirectionRequest{
		WatcherName: "",
		TargetPath:  "/tmp/out",
	})

	assertCode(t, err, codes.InvalidArgument)
}

func TestAddRedirection_MissingTargetPath(t *testing.T) {
	svc := newService(&mocks.MockRedirectionRepository{})

	_, err := svc.CreateFileRedirection(ctx, &pb.CreateFileRedirectionRequest{
		WatcherName: "my-watcher",
		TargetPath:  "",
	})

	assertCode(t, err, codes.InvalidArgument)
}

func TestAddRedirection_RepositoryError(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	repo.On("AddRedirection", "my-watcher", "/tmp/out", false).Return(nil, errors.New("db error"))

	_, err := svc.CreateFileRedirection(ctx, &pb.CreateFileRedirectionRequest{
		WatcherName: "my-watcher",
		TargetPath:  "/tmp/out",
	})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

// ── GetRedirection ────────────────────────────────────────────────────────────

func TestGetRedirection_OK(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	r := newRedirection("my-watcher", "/tmp/out", false)
	repo.On("GetRedirection", "my-watcher").Return(r, nil)

	resp, err := svc.GetFileRedirection(ctx, &pb.GetFileRedirectionRequest{Name: "my-watcher"})

	assert.NoError(t, err)
	assert.Equal(t, "my-watcher", resp.WatcherName)
	assert.Equal(t, "/tmp/out", resp.TargetPath)
	repo.AssertExpectations(t)
}

func TestGetRedirection_NotFound(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	repo.On("GetRedirection", "unknown").Return(nil, errors.New("not found"))

	_, err := svc.GetFileRedirection(ctx, &pb.GetFileRedirectionRequest{Name: "unknown"})

	assertCode(t, err, codes.NotFound)
	repo.AssertExpectations(t)
}

// ── UpdateFileRedirection ─────────────────────────────────────────────────────

func TestUpdateFileRedirection_OK(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	newPath := "/tmp/new-out"
	r := newRedirection("my-watcher", newPath, true)
	repo.On("UpdateRedirection", "my-watcher", &newPath, (*bool)(nil)).Return(r, nil)

	resp, err := svc.UpdateFileRedirection(ctx, &pb.UpdateFileRedirectionRequest{
		WatcherName: "my-watcher",
		TargetPath:  &newPath,
	})

	assert.NoError(t, err)
	assert.Equal(t, newPath, resp.TargetPath)
	repo.AssertExpectations(t)
}

func TestUpdateFileRedirection_MissingParameters(t *testing.T) {
	svc := newService(&mocks.MockRedirectionRepository{})

	_, err := svc.UpdateFileRedirection(ctx, &pb.UpdateFileRedirectionRequest{WatcherName: "my-watcher", TargetPath: nil, AutoFlush: nil})

	assertCode(t, err, codes.InvalidArgument)
}

func TestUpdateFileRedirection_MissingWatcherName(t *testing.T) {
	svc := newService(&mocks.MockRedirectionRepository{})

	newPath := "/tmp/new-out"
	_, err := svc.UpdateFileRedirection(ctx, &pb.UpdateFileRedirectionRequest{
		WatcherName: "",
		TargetPath:  &newPath,
	})

	assertCode(t, err, codes.InvalidArgument)
}

func TestUpdateFileRedirection_MissingTargetPath(t *testing.T) {
	svc := newService(&mocks.MockRedirectionRepository{})

	_, err := svc.UpdateFileRedirection(ctx, &pb.UpdateFileRedirectionRequest{
		WatcherName: "my-watcher",
		TargetPath:  nil,
	})

	assertCode(t, err, codes.InvalidArgument)
}

func TestUpdateFileRedirection_RepositoryError(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	newPath := "/tmp/new-out"
	repo.On("UpdateRedirection", "my-watcher", &newPath, (*bool)(nil)).Return(nil, errors.New("db error"))

	_, err := svc.UpdateFileRedirection(ctx, &pb.UpdateFileRedirectionRequest{
		WatcherName: "my-watcher",
		TargetPath:  &newPath,
	})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}

// ── DeleteFileRedirection ─────────────────────────────────────────────────────

func TestDeleteFileRedirection_OK(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	repo.On("RemoveRedirection", "my-watcher").Return(nil)

	resp, err := svc.DeleteFileRedirection(ctx, &pb.DeleteFileRedirectionRequest{WatcherName: "my-watcher"})

	assert.NoError(t, err)
	assert.True(t, resp.Success)
	repo.AssertExpectations(t)
}

func TestDeleteFileRedirection_RepositoryError(t *testing.T) {
	repo := &mocks.MockRedirectionRepository{}
	svc := newService(repo)

	repo.On("RemoveRedirection", "my-watcher").Return(errors.New("not found"))

	_, err := svc.DeleteFileRedirection(ctx, &pb.DeleteFileRedirectionRequest{WatcherName: "my-watcher"})

	assertCode(t, err, codes.Internal)
	repo.AssertExpectations(t)
}
