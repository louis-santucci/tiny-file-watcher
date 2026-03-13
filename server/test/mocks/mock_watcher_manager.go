package mocks

import (
	"tiny-file-watcher/server/watcher"

	"github.com/stretchr/testify/mock"
)

// MockWatcherManager is a testify mock for watcher.WatcherManager.
type MockWatcherManager struct {
	mock.Mock
}

func (m *MockWatcherManager) Start(key watcher.WatcherKey, sourcePath string) error {
	args := m.Called(key, sourcePath)
	return args.Error(0)
}

func (m *MockWatcherManager) Stop(key watcher.WatcherKey) {
	m.Called(key)
}

func (m *MockWatcherManager) IsRunning(key watcher.WatcherKey) bool {
	args := m.Called(key)
	return args.Bool(0)
}
