package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const watcherNotFound = "watcher %s not found"

const schema = `
CREATE TABLE IF NOT EXISTS file_watchers (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	source_path TEXT NOT NULL,
	enabled     INTEGER NOT NULL DEFAULT 0,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS watched_files (
	id          TEXT PRIMARY KEY,
	watcher_id  TEXT NOT NULL REFERENCES file_watchers(id) ON DELETE CASCADE,
	file_path   TEXT NOT NULL,
	detected_at TEXT NOT NULL
);
`

// DB wraps a SQLite connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := conn.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Enable foreign key enforcement.
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error { return db.conn.Close() }

// --- FileWatcher ---

// FileWatcher mirrors the file_watchers table row.
type FileWatcher struct {
	ID         string
	Name       string
	SourcePath string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateWatcher inserts a new FileWatcher and returns it.
func (db *DB) CreateWatcher(name, sourcePath string) (*FileWatcher, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := db.conn.Exec(
		`INSERT INTO file_watchers (id, name, source_path, enabled, created_at, updated_at) VALUES (?,?,?,0,?,?)`,
		id, name, sourcePath, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	return &FileWatcher{ID: id, Name: name, SourcePath: sourcePath, Enabled: false, CreatedAt: now, UpdatedAt: now}, nil
}

// GetWatcher returns a FileWatcher by ID.
func (db *DB) GetWatcher(id string) (*FileWatcher, error) {
	row := db.conn.QueryRow(`SELECT id, name, source_path, enabled, created_at, updated_at FROM file_watchers WHERE id = ?`, id)
	return scanWatcher(row)
}

// ListWatchers returns all FileWatchers.
func (db *DB) ListWatchers() ([]*FileWatcher, error) {
	rows, err := db.conn.Query(`SELECT id, name, source_path, enabled, created_at, updated_at FROM file_watchers`)
	if err != nil {
		return nil, fmt.Errorf("list watchers: %w", err)
	}
	defer rows.Close()
	var result []*FileWatcher
	for rows.Next() {
		w, err := scanWatcher(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

// UpdateWatcher updates name and source_path for the given ID.
func (db *DB) UpdateWatcher(id, name, sourcePath string) (*FileWatcher, error) {
	now := time.Now().UTC()
	res, err := db.conn.Exec(
		`UPDATE file_watchers SET name=?, source_path=?, updated_at=? WHERE id=?`,
		name, sourcePath, now.Format(time.RFC3339), id,
	)
	if err != nil {
		return nil, fmt.Errorf("update watcher: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf(watcherNotFound, id)
	}
	return db.GetWatcher(id)
}

// DeleteWatcher removes a FileWatcher (and its watched_files via cascade).
func (db *DB) DeleteWatcher(id string) error {
	res, err := db.conn.Exec(`DELETE FROM file_watchers WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete watcher: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf(watcherNotFound, id)
	}
	return nil
}

// ToggleWatcher flips the enabled state and returns the updated watcher.
func (db *DB) ToggleWatcher(id string) (*FileWatcher, error) {
	now := time.Now().UTC()
	res, err := db.conn.Exec(
		`UPDATE file_watchers SET enabled = 1 - enabled, updated_at=? WHERE id=?`,
		now.Format(time.RFC3339), id,
	)
	if err != nil {
		return nil, fmt.Errorf("toggle watcher: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf(watcherNotFound, id)
	}
	return db.GetWatcher(id)
}

// ListEnabledWatchers returns all watchers with enabled=1.
func (db *DB) ListEnabledWatchers() ([]*FileWatcher, error) {
	rows, err := db.conn.Query(`SELECT id, name, source_path, enabled, created_at, updated_at FROM file_watchers WHERE enabled=1`)
	if err != nil {
		return nil, fmt.Errorf("list enabled watchers: %w", err)
	}
	defer rows.Close()
	var result []*FileWatcher
	for rows.Next() {
		w, err := scanWatcher(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanWatcher(s scanner) (*FileWatcher, error) {
	var w FileWatcher
	var enabledInt int
	var createdStr, updatedStr string
	if err := s.Scan(&w.ID, &w.Name, &w.SourcePath, &enabledInt, &createdStr, &updatedStr); err != nil {
		return nil, fmt.Errorf("scan watcher: %w", err)
	}
	w.Enabled = enabledInt == 1
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	w.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &w, nil
}

// --- WatchedFile ---

// WatchedFile mirrors the watched_files table row.
type WatchedFile struct {
	ID         string
	WatcherID  string
	FilePath   string
	DetectedAt time.Time
}

// AddWatchedFile inserts a newly detected file.
func (db *DB) AddWatchedFile(watcherID, filePath string) (*WatchedFile, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	_, err := db.conn.Exec(
		`INSERT INTO watched_files (id, watcher_id, file_path, detected_at) VALUES (?,?,?,?)`,
		id, watcherID, filePath, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("add watched file: %w", err)
	}
	return &WatchedFile{ID: id, WatcherID: watcherID, FilePath: filePath, DetectedAt: now}, nil
}

// RemoveWatchedFile deletes the entry for a given watcher and file path.
func (db *DB) RemoveWatchedFile(watcherID, filePath string) error {
	_, err := db.conn.Exec(`DELETE FROM watched_files WHERE watcher_id=? AND file_path=?`, watcherID, filePath)
	return err
}
