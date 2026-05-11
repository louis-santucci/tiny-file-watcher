package database

import (
	"tiny-file-watcher/internal"
)

// FileRepository defines the persistence operations for WatchedFile entities.
type FileRepository interface {
	AddWatchedFile(watcherName string, filePath string, flushed bool) (*WatchedFile, error)
	BulkAddWatchedFiles(watcherName string, files *internal.Set[string], flushed bool) ([]*WatchedFile, error)
	RemoveWatchedFile(watcherName string, filePath string) error
	BulkRemoveWatchedFiles(watcherName string, filePaths []string) error
	FlushWatchedFiles(ids []int64) error
	ListWatchedFiles(watcherName string) ([]*WatchedFile, error)
}

// Compile-time assertion: *database.DB must satisfy FileRepository.
var _ FileRepository = (*DB)(nil)
