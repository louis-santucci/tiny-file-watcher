package watcher

import "tiny-file-watcher/server/database"

// FileWatcherRepository defines the persistence operations for FileWatcher entities.
type FileWatcherRepository interface {
	CreateWatcher(name, sourcePath, machineName string) (*database.FileWatcher, error)
	GetWatcherById(id int64) (*database.FileWatcher, error)
	GetWatcherByName(name string) (*database.FileWatcher, error)
	ListWatchers() ([]*database.FileWatcher, error)
	ListWatchersByMachine(machineName string) ([]*database.FileWatcher, error)
	UpdateWatcher(id int64, name *string, sourcePath *string) (*database.FileWatcher, error)
	DeleteWatcher(name string) error
}

// Compile-time assertion: *database.DB must satisfy FileWatcherRepository.
var _ FileWatcherRepository = (*database.DB)(nil)
