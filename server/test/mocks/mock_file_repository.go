package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockFileRepository is a testify mock for watcher.FileRepository.
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) AddWatchedFile(watcherName string, filePath string, flushed bool) (*database.WatchedFile, error) {
	args := m.Called(watcherName, filePath, flushed)
	if v := args.Get(0); v != nil {
		return v.(*database.WatchedFile), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileRepository) RemoveWatchedFile(watcherName string, filePath string) error {
	args := m.Called(watcherName, filePath)
	return args.Error(0)
}

func (m *MockFileRepository) FlushWatchedFiles(ids []int64) error {
	args := m.Called(ids)
	return args.Error(0)
}

func (m *MockFileRepository) ListWatchedFiles(watcherName string) ([]*database.WatchedFile, error) {
	args := m.Called(watcherName)
	if v := args.Get(0); v != nil {
		return v.([]*database.WatchedFile), args.Error(1)
	}
	return nil, args.Error(1)
}
