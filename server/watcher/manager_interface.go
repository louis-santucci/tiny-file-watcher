package watcher

// WatcherManager abstracts the goroutine lifecycle for file watchers.
type WatcherManager interface {
	Start(key WatcherKey, sourcePath string) error
	Stop(key WatcherKey)
	IsRunning(key WatcherKey) bool
}

// Compile-time assertion: *Manager must satisfy WatcherManager.
var _ WatcherManager = (*Manager)(nil)
