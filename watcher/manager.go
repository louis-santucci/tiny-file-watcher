package watcher

import (
	"log/slog"
	"sync"

	"tiny-file-watcher/database"

	"github.com/fsnotify/fsnotify"
)

// Manager manages one fsnotify goroutine per enabled FileWatcher.
type Manager struct {
	mu       sync.Mutex
	watchers map[string]*fsnotify.Watcher // watcher ID → fsnotify handle
	db       *database.DB
}

// NewManager creates a new Manager backed by the given database.
func NewManager(db *database.DB) *Manager {
	return &Manager{
		watchers: make(map[string]*fsnotify.Watcher),
		db:       db,
	}
}

// Start begins watching sourcePath for the given watcher ID.
// It is a no-op if the watcher is already running.
func (m *Manager) Start(id, sourcePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, running := m.watchers[id]; running {
		slog.Warn("watcher already running", "watcher_id", id)
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

	m.watchers[id] = fw
	go m.loop(id, fw)
	slog.Info("watcher started", "watcher_id", id, "path", sourcePath)
	return nil
}

// Stop halts the goroutine for the given watcher ID.
// It is a no-op if the watcher is not running.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fw, ok := m.watchers[id]; ok {
		fw.Close()
		delete(m.watchers, id)
		slog.Info("watcher stopped", "watcher_id", id)
	}
}

// IsRunning reports whether a goroutine is active for the given ID.
func (m *Manager) IsRunning(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.watchers[id]
	return ok
}

// loop processes fsnotify events until the watcher is closed.
func (m *Manager) loop(id string, fw *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-fw.Events:
			if !ok {
				return
			}
			switch {
			case event.Has(fsnotify.Create):
				if _, err := m.db.AddWatchedFile(id, event.Name); err != nil {
					slog.Error("error adding file", "watcher_id", id, "event", event.Name, "err", err)
				} else {
					slog.Debug("file created", "watcher_id", id, "event", event.Name)
				}
			case event.Has(fsnotify.Remove):
				if err := m.db.RemoveWatchedFile(id, event.Name); err != nil {
					slog.Error("error removing file", "watcher_id", id, "event", event.Name, "err", err)

				} else {
					slog.Debug("file deleted", "watcher_id", id, "event", event.Name)
				}
				// Write and Rename events are intentionally ignored.
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "watcher_id", id, "err", err)
		}
	}
}
