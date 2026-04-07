package mocks

import (
	"context"
	"tiny-file-watcher/server/database"

	"github.com/stretchr/testify/mock"
)

// MockTransactor is a testify mock for database.Transactor.
type MockTransactor struct {
	mock.Mock
}

func (m *MockTransactor) WithTransaction(ctx context.Context, fn func(repository database.TransactionalFileRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}
