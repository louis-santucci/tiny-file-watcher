package watcher

import "tiny-file-watcher/server/database"

// FileRepository defines the persistence operations for WatchedFile entities.
type FileRepository interface {
	AddWatchedFile(watcherID int64, filePath string) (*database.WatchedFile, error)
	RemoveWatchedFile(watcherID int64, filePath string) error
}

// Compile-time assertion: *database.DB must satisfy FileRepository.
var _ FileRepository = (*database.DB)(nil)
