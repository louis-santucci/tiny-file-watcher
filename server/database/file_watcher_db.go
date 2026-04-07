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
	ID          int64
	MachineName string
	Name        string
	SourcePath  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateWatcher inserts a new FileWatcher associated with a machine and returns it.
func (db *DB) CreateWatcher(name, sourcePath, machineName string) (*FileWatcher, error) {
	now := time.Now().UTC()
	created, err := db.conn.Exec(
		`INSERT INTO file_watchers (machine_name, name, source_path, created_at, updated_at) VALUES (?,?,?,?,?)`,
		machineName, name, sourcePath, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	createdId, err := created.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get created id: %w", err)
	}
	return &FileWatcher{ID: createdId, MachineName: machineName, Name: name, SourcePath: sourcePath, CreatedAt: now, UpdatedAt: now}, nil
}

// GetWatcherById returns a FileWatcher by ID.
func (db *DB) GetWatcherById(id int64) (*FileWatcher, error) {
	row := db.conn.QueryRow(`SELECT id, machine_name, name, source_path, created_at, updated_at FROM file_watchers WHERE id = ?`, id)
	return scanWatcher(row)
}

// GetWatcherByName returns a FileWatcher by name.
func (db *DB) GetWatcherByName(name string) (*FileWatcher, error) {
	row := db.conn.QueryRow(`SELECT id, machine_name, name, source_path, created_at, updated_at FROM file_watchers WHERE name = ?`, name)
	return scanWatcher(row)
}

// ListWatchers returns all FileWatchers.
func (db *DB) ListWatchers() ([]*FileWatcher, error) {
	rows, err := db.conn.Query(`SELECT id, machine_name, name, source_path, created_at, updated_at FROM file_watchers`)
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

// ListWatchersByMachine returns all FileWatchers belonging to a specific machine.
func (db *DB) ListWatchersByMachine(machineName string) ([]*FileWatcher, error) {
	rows, err := db.conn.Query(
		`SELECT id, machine_name, name, source_path, created_at, updated_at FROM file_watchers WHERE machine_name = ?`,
		machineName,
	)
	if err != nil {
		return nil, fmt.Errorf("list watchers by machine: %w", err)
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
func (db *DB) UpdateWatcher(id int64, name *string, sourcePath *string) (*FileWatcher, error) {
	if name == nil && sourcePath == nil {
		return db.GetWatcherById(id)
	}

	now := time.Now().UTC()
	setClauses := []string{}
	args := []any{}

	if name != nil {
		setClauses = append(setClauses, "name=?")
		args = append(args, name)
	}
	if sourcePath != nil {
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

func scanWatcher(s scanner) (*FileWatcher, error) {
	var w FileWatcher
	var createdStr, updatedStr string
	if err := s.Scan(&w.ID, &w.MachineName, &w.Name, &w.SourcePath, &createdStr, &updatedStr); err != nil {
		return nil, fmt.Errorf("scan watcher: %w", err)
	}
	w.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	w.UpdatedAt, _ = time.Parse(time.RFC3339, updatedStr)
	return &w, nil
}
