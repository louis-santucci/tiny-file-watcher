package mocks

import (
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockRedirectionRepository is a testify mock for redirection.RedirectionRepository.
type MockRedirectionRepository struct {
	mock.Mock
}

func (m *MockRedirectionRepository) AddRedirection(watcherName string, targetPath string, autoFlush bool) (*database.FileRedirection, error) {
	args := m.Called(watcherName, targetPath, autoFlush)
	if v := args.Get(0); v != nil {
		return v.(*database.FileRedirection), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRedirectionRepository) GetRedirection(watcherName string) (*database.FileRedirection, error) {
	args := m.Called(watcherName)
	if v := args.Get(0); v != nil {
		return v.(*database.FileRedirection), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRedirectionRepository) RemoveRedirection(watcherName string) error {
	args := m.Called(watcherName)
	return args.Error(0)
}

func (m *MockRedirectionRepository) UpdateRedirection(watcherName string, filePath *string, autoFlush *bool) (*database.FileRedirection, error) {
	args := m.Called(watcherName, filePath, autoFlush)
	if v := args.Get(0); v != nil {
		return v.(*database.FileRedirection), args.Error(1)
	}
	return nil, args.Error(1)
}
