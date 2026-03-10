package watcher

import (
	"log/slog"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Manager manages one fsnotify goroutine per enabled FileWatcher.
type Manager struct {
	mu             sync.Mutex
	watchers       map[WatcherKey]*fsnotify.Watcher // watcher ID → fsnotify handle
	fileRepository FileRepository
}

type WatcherKey struct {
	Id   int64
	Name string
}

// NewManager creates a new Manager backed by the given FileRepository.
func NewManager(fileRepository FileRepository) *Manager {
	return &Manager{
		watchers:       make(map[WatcherKey]*fsnotify.Watcher),
		fileRepository: fileRepository,
	}
}

// Start begins watching sourcePath for the given watcher ID.
// It is a no-op if the watcher is already running.
func (m *Manager) Start(key WatcherKey, sourcePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, running := m.watchers[key]; running {
		slog.Warn("watcher already running", "watcher_name", key.Name, "watcher_id", key.Id)
		return nil
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := fw.Add(sourcePath); err != nil {
		fw.Close()
		return err
	}

	m.watchers[key] = fw
	go m.loop(key, fw)
	slog.Info("watcher started", "watcher_name", key.Name, "watcher_id", key.Id, "path", sourcePath)
	return nil
}

// Stop halts the goroutine for the given watcher ID.
// It is a no-op if the watcher is not running.
func (m *Manager) Stop(key WatcherKey) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fw, ok := m.watchers[key]; ok {
		fw.Close()
		delete(m.watchers, key)
		slog.Info("watcher stopped", "watcher_name", key.Name, "watcher_id", key.Id, "watcher_name", key.Name)
	}
}

// IsRunning reports whether a goroutine is active for the given ID.
func (m *Manager) IsRunning(key WatcherKey) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.watchers[key]
	return ok
}

// loop processes fsnotify events until the watcher is closed.
func (m *Manager) loop(key WatcherKey, fw *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-fw.Events:
			if !ok {
				return
			}
			switch {
			case event.Has(fsnotify.Create):
				if _, err := m.fileRepository.AddWatchedFile(key.Name, event.Name, false); err != nil {
					slog.Error("error adding file", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name, "err", err)
				} else {
					slog.Debug("file created", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name)
				}
			case event.Has(fsnotify.Remove):
				if err := m.fileRepository.RemoveWatchedFile(key.Name, event.Name); err != nil {
					slog.Error("error removing file", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name, "err", err)

				} else {
					slog.Debug("file deleted", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name)
				}
				// Write and Rename events are intentionally ignored.
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "watcher_name", key.Name, "watcher_id", key.Id, "err", err)
		}
	}
}
