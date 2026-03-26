package watcher

import "tiny-file-watcher/server/database"

// FileRepository defines the persistence operations for WatchedFile entities.
type FileRepository interface {
	AddWatchedFile(watcherName string, filePath string, flushed bool) (*database.WatchedFile, error)
	RemoveWatchedFile(watcherName string, filePath string) error
	FlushWatchedFiles(ids []int64) error
	ListWatchedFiles(watcherName string) ([]*database.WatchedFile, error)
}

// Compile-time assertion: *database.DB must satisfy FileRepository.
var _ FileRepository = (*database.DB)(nil)
