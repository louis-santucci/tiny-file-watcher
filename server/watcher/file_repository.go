package watcher

import "tiny-file-watcher/server/database"

// FileRepository defines the persistence operations for WatchedFile entities.
type FileRepository interface {
	AddWatchedFile(watcherID int64, filePath string, flushed bool) (*database.WatchedFile, error)
	RemoveWatchedFile(watcherID int64, filePath string) error
	FlushWatchedFiles(ids []int64) error
}

// Compile-time assertion: *database.DB must satisfy FileRepository.
var _ FileRepository = (*database.DB)(nil)
