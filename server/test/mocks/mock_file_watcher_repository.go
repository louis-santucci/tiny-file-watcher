package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockFileWatcherRepository is a testify mock for watcher.FileWatcherRepository.
type MockFileWatcherRepository struct {
	mock.Mock
}

func (m *MockFileWatcherRepository) CreateWatcher(name, sourcePath string) (*database.FileWatcher, error) {
	args := m.Called(name, sourcePath)
	if v := args.Get(0); v != nil {
		return v.(*database.FileWatcher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileWatcherRepository) GetWatcherById(id int64) (*database.FileWatcher, error) {
	args := m.Called(id)
	if v := args.Get(0); v != nil {
		return v.(*database.FileWatcher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileWatcherRepository) GetWatcherByName(name string) (*database.FileWatcher, error) {
	args := m.Called(name)
	if v := args.Get(0); v != nil {
		return v.(*database.FileWatcher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileWatcherRepository) ListWatchers() ([]*database.FileWatcher, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]*database.FileWatcher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileWatcherRepository) UpdateWatcher(id int64, name *string, sourcePath *string) (*database.FileWatcher, error) {
	args := m.Called(id, name, sourcePath)
	if v := args.Get(0); v != nil {
		return v.(*database.FileWatcher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFileWatcherRepository) DeleteWatcher(name string) error {
	args := m.Called(name)
	return args.Error(0)
}
