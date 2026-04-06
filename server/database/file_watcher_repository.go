package database

// FileWatcherRepository defines the persistence operations for FileWatcher entities.
type FileWatcherRepository interface {
	CreateWatcher(name, sourcePath, machineName string) (*FileWatcher, error)
	GetWatcherById(id int64) (*FileWatcher, error)
	GetWatcherByName(name string) (*FileWatcher, error)
	ListWatchers() ([]*FileWatcher, error)
	ListWatchersByMachine(machineName string) ([]*FileWatcher, error)
	UpdateWatcher(id int64, name *string, sourcePath *string) (*FileWatcher, error)
	DeleteWatcher(name string) error
}

// Compile-time assertion: *database.DB must satisfy FileWatcherRepository.
var _ FileWatcherRepository = (*DB)(nil)
