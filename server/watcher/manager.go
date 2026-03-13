package watcher

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Manager manages one fsnotify goroutine per enabled FileWatcher.
type Manager struct {
	mu               sync.Mutex
	watchers         map[WatcherKey]*fsnotify.Watcher // watcher ID → fsnotify handle
	fileRepository   FileRepository
	filterRepository FilterRepository
	logger           *slog.Logger
}

type WatcherKey struct {
	Id   int64
	Name string
}

// NewManager creates a new Manager backed by the given FileRepository and FilterRepository.
func NewManager(fileRepository FileRepository, filterRepository FilterRepository, logger *slog.Logger) *Manager {
	return &Manager{
		watchers:         make(map[WatcherKey]*fsnotify.Watcher),
		fileRepository:   fileRepository,
		filterRepository: filterRepository,
		logger:           logger,
	}
}

// Start begins watching sourcePath for the given watcher ID.
// It is a no-op if the watcher is already running.
func (m *Manager) Start(key WatcherKey, sourcePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, running := m.watchers[key]; running {
		m.logger.Warn("watcher already running", "watcher_name", key.Name, "watcher_id", key.Id)
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
	// Also watch all existing subdirectories.
	if err := addSubdirs(fw, sourcePath); err != nil {
		fw.Close()
		return err
	}

	m.watchers[key] = fw
	go m.loop(key, fw)
	m.logger.Info("watcher started", "watcher_name", key.Name, "watcher_id", key.Id, "path", sourcePath)
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
		m.logger.Info("watcher stopped", "watcher_name", key.Name, "watcher_id", key.Id, "watcher_name", key.Name)
	}
}

// IsRunning reports whether a goroutine is active for the given ID.
func (m *Manager) IsRunning(key WatcherKey) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.watchers[key]
	return ok
}

// addSubdirs walks root and registers every subdirectory with fw.
func addSubdirs(fw *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() && path != root {
			return fw.Add(path)
		}
		return nil
	})
}

func (m *Manager) loop(key WatcherKey, fw *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-fw.Events:
			if !ok {
				return
			}
			switch {
			case event.Has(fsnotify.Create):
				m.handleCreateEvent(key, event, fw)
				break
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				m.handleRemoveEvent(key, event, fw)
				break
			}
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			m.logger.Error("watcher error", "watcher_name", key.Name, "watcher_id", key.Id, "err", err)
		}
	}
}

func (m *Manager) handleCreateEvent(key WatcherKey, event fsnotify.Event, fw *fsnotify.Watcher) {
	info, err := os.Stat(event.Name)
	if err != nil {
		return
	}
	if info.IsDir() {
		if addErr := fw.Add(event.Name); addErr != nil {
			m.logger.Error("error watching new directory", "watcher_name", key.Name, "watcher_id", key.Id, "dir", event.Name, "err", addErr)
		}
		return
	}
	filters, err := m.filterRepository.GetFiltersForWatcher(key.Name)
	if err != nil {
		m.logger.Error("error loading filters", "watcher_name", key.Name, "watcher_id", key.Id, "err", err)
		return
	}
	if !Evaluate(filters, event.Name) {
		m.logger.Debug("file rejected by filter", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name)
		return
	}
	if _, err := m.fileRepository.AddWatchedFile(key.Name, event.Name, false); err != nil {
		m.logger.Error("error adding file", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name, "err", err)
	} else {
		m.logger.Debug("file created", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name)
	}
}

func (m *Manager) handleRemoveEvent(key WatcherKey, event fsnotify.Event, fw *fsnotify.Watcher) {
	if err := m.fileRepository.RemoveWatchedFile(key.Name, event.Name); err != nil {
		m.logger.Error("error removing file", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name, "err", err)

	} else {
		m.logger.Debug("file deleted", "watcher_name", key.Name, "watcher_id", key.Id, "event", event.Name)
	}
	// Write and Rename events are intentionally ignored.
}
