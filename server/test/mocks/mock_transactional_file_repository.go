package mocks

import (
	"context"
	"tiny-file-watcher/internal"
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockTransactionalFileRepository is a testify mock for database.TransactionalFileRepository.
type MockTransactionalFileRepository struct {
	mock.Mock
}

func (m *MockTransactionalFileRepository) BulkAddWatchedFiles(watcherName string, files *internal.Set[string], flushed bool) ([]*database.WatchedFile, error) {
	args := m.Called(watcherName, files, flushed)
	if v := args.Get(0); v != nil {
		return v.([]*database.WatchedFile), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTransactionalFileRepository) BulkRemoveWatchedFiles(watcherName string, filePaths []string) error {
	args := m.Called(watcherName, filePaths)
	return args.Error(0)
}

// PassthroughTransactor is a Transactor that executes the given function
// with the provided TransactionalFileRepository.  Use it in tests that need
// the sync logic to actually persist files (or verify the repository calls).
type PassthroughTransactor struct {
	Repo database.TransactionalFileRepository
}

func (p *PassthroughTransactor) WithTransaction(_ context.Context, fn func(database.TransactionalFileRepository) error) error {
	return fn(p.Repo)
}

// NoopTransactor is a Transactor that executes the given function with a
// no-op TransactionalFileRepository.  Use it in tests where no files are
// expected to be added or removed (so the repository is never actually called
// inside the transaction).
type NoopTransactor struct{}

func (NoopTransactor) WithTransaction(_ context.Context, fn func(database.TransactionalFileRepository) error) error {
	return fn(&noopTransactionalFileRepository{})
}

type noopTransactionalFileRepository struct{}

func (*noopTransactionalFileRepository) BulkAddWatchedFiles(_ string, _ *internal.Set[string], _ bool) ([]*database.WatchedFile, error) {
	return nil, nil
}

func (*noopTransactionalFileRepository) BulkRemoveWatchedFiles(_ string, _ []string) error {
	return nil
}
