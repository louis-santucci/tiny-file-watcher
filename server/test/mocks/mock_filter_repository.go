package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockFilterRepository is a testify mock for watcher.FilterRepository.
type MockFilterRepository struct {
	mock.Mock
}

func (m *MockFilterRepository) AddFilter(watcherName, ruleType, patternType, pattern string) (*database.WatcherFilter, error) {
	args := m.Called(watcherName, ruleType, patternType, pattern)
	if v := args.Get(0); v != nil {
		return v.(*database.WatcherFilter), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFilterRepository) GetFiltersForWatcher(watcherName string) ([]*database.WatcherFilter, error) {
	args := m.Called(watcherName)
	if v := args.Get(0); v != nil {
		return v.([]*database.WatcherFilter), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFilterRepository) ListFilters() ([]*database.WatcherFilter, error) {
	args := m.Called()
	if v := args.Get(0); v != nil {
		return v.([]*database.WatcherFilter), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockFilterRepository) DeleteFilter(id int64) error {
	args := m.Called(id)
	return args.Error(0)
}
