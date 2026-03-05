package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockFileRepository is a testify mock for watcher.FileRepository.
type MockFileRepository struct {
	mock.Mock
}

func (m *MockFileRepository) AddWatchedFile(watcherID int64, filePath string) (*database.WatchedFile, error) {
	args := m.Called(watcherID, filePath)
	if v := args.Get(0); v != nil {
		return v.(*database.WatchedFile), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileRepository) RemoveWatchedFile(watcherID int64, filePath string) error {
	args := m.Called(watcherID, filePath)
	return args.Error(0)
}
