package watcher

import "tiny-file-watcher/server/database"

// FilterRepository defines the persistence operations for WatcherFilter entities.
type FilterRepository interface {
	AddFilter(watcherName, ruleType, patternType, pattern string) (*database.WatcherFilter, error)
	GetFiltersForWatcher(watcherName string) ([]*database.WatcherFilter, error)
	ListFilters() ([]*database.WatcherFilter, error)
	DeleteFilter(id int64) error
}

// Compile-time assertion: *database.DB must satisfy FilterRepository.
var _ FilterRepository = (*database.DB)(nil)
