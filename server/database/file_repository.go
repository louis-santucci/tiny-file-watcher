package database

import (
	"context"
)

// FileRepository defines the persistence operations for WatchedFile entities.
type FileRepository interface {
	AddWatchedFile(watcherName string, filePath string, flushed bool) (*WatchedFile, error)
	RemoveWatchedFile(watcherName string, filePath string) error
	FlushWatchedFiles(ids []int64) error
	ListWatchedFiles(watcherName string) ([]*WatchedFile, error)
}

type TransactionalFileRepository interface {
	BulkAddWatchedFiles(watcherName string, files map[string]string, flushed bool) ([]*WatchedFile, error)
	BulkRemoveWatchedFiles(watcherName string, filePaths []string) error
}

type Transactor interface {
	WithTransaction(ctx context.Context, fn func(repository TransactionalFileRepository) error) error
}

// Compile-time assertion: *database.DB must satisfy FileRepository.
var _ FileRepository = (*DB)(nil)

var _ Transactor = (*DB)(nil)
