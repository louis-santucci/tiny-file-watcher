package watcher

import (
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"tiny-file-watcher/database"
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
					log.Printf("watcher %s: add file %q: %v", id, event.Name, err)
				} else {
					log.Printf("watcher %s: created %q", id, event.Name)
				}
			case event.Has(fsnotify.Remove):
				if err := m.db.RemoveWatchedFile(id, event.Name); err != nil {
					log.Printf("watcher %s: remove file %q: %v", id, event.Name, err)
				} else {
					log.Printf("watcher %s: removed %q", id, event.Name)
				}
				// Write and Rename events are intentionally ignored.
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			log.Printf("watcher %s error: %v", id, err)
		}
	}
}
