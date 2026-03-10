package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockFlushRepository is a testify mock for flush.FlushRepository.
type MockFlushRepository struct {
	mock.Mock
}

func (m *MockFlushRepository) ListPendingFlushes(watcherName string) ([]*database.PendingFlush, error) {
	args := m.Called(watcherName)
	if v := args.Get(0); v != nil {
		return v.([]*database.PendingFlush), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFlushRepository) FlushWatchedFiles(ids []int64) error {
	args := m.Called(ids)
	return args.Error(0)
}
