package database

import (
	"fmt"
	"strings"
	"time"
)

const watcherNotFound = "watcher %s not found"
const watcherIdNotFound = "watcher with id %d not found"

// FileWatcher mirrors the file_watchers table row.
type FileWatcher struct {
	ID         int64
	Name       string
	SourcePath string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateWatcher inserts a new FileWatcher and returns it.
func (db *DB) CreateWatcher(name, sourcePath string) (*FileWatcher, error) {
	now := time.Now().UTC()
	created, err := db.conn.Exec(
		`INSERT INTO file_watchers (name, source_path, enabled, created_at, updated_at) VALUES (?,?,0,?,?)`,
		name, sourcePath, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	createdId, err := created.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get created id: %w", err)
	}
	return &FileWatcher{ID: createdId, Name: name, SourcePath: sourcePath, Enabled: false, CreatedAt: now, UpdatedAt: now}, nil
}

// GetWatcherById returns a FileWatcher by ID.
func (db *DB) GetWatcherById(id int64) (*FileWatcher, error) {
	row := db.conn.QueryRow(`SELECT id, name, source_path, enabled, created_at, updated_at FROM file_watchers WHERE id = ?`, id)
	return scanWatcher(row)
}

// GetWatcherByName returns a FileWatcher by name.
func (db *DB) GetWatcherByName(name string) (*FileWatcher, error) {
	row := db.conn.QueryRow(`SELECT id, name, source_path, enabled, created_at, updated_at FROM file_watchers WHERE name = ?`, name)
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
func (db *DB) UpdateWatcher(id int64, name string, sourcePath string) (*FileWatcher, error) {
	if name == "" && sourcePath == "" {
		return db.GetWatcherById(id)
	}

	now := time.Now().UTC()
	setClauses := []string{}
	args := []any{}

	if name != "" {
		setClauses = append(setClauses, "name=?")
		args = append(args, name)
	}
	if sourcePath != "" {
		setClauses = append(setClauses, "source_path=?")
		args = append(args, sourcePath)
	}
	setClauses = append(setClauses, "updated_at=?")
	args = append(args, now.Format(time.RFC3339))
	args = append(args, id)

	query := fmt.Sprintf("UPDATE file_watchers SET %s WHERE id=?", strings.Join(setClauses, ", "))
	res, err := db.conn.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update watcher: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf(watcherIdNotFound, id)
	}
	return db.GetWatcherById(id)
}

// DeleteWatcher removes a FileWatcher (and its watched_files via cascade).
func (db *DB) DeleteWatcher(name string) error {
	res, err := db.conn.Exec(`DELETE FROM file_watchers WHERE name=?`, name)
	if err != nil {
		return fmt.Errorf("delete watcher: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf(watcherNotFound, name)
	}
	return nil
}

// ToggleWatcher flips the enabled state and returns the updated watcher.
func (db *DB) ToggleWatcher(name string) (*FileWatcher, error) {
	now := time.Now().UTC()
	res, err := db.conn.Exec(
		`UPDATE file_watchers SET enabled = 1 - enabled, updated_at=? WHERE name=?`,
		now.Format(time.RFC3339), name,
	)
	if err != nil {
		return nil, fmt.Errorf("toggle watcher: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf(watcherNotFound, name)
	}
	return db.GetWatcherByName(name)
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
