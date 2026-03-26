package database

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

// WatchedFile mirrors the watched_files table row.
type WatchedFile struct {
	ID          int64
	WatcherName string
	FilePath    string
	Flushed     bool
	DetectedAt  time.Time
}

// PendingFlush mirrors one row of the pending_file_flushes view.
type PendingFlush struct {
	WatchedFileID int64
	WatcherName   string
	FilePath      string
	FileName      string
	TargetPath    string
}

// AddWatchedFile inserts a newly detected file.
// filePath is the full path to the file; file_path in the DB stores only the parent directory.
func (db *DB) AddWatchedFile(watcherName string, filePath string, flushed bool) (*WatchedFile, error) {
	now := time.Now().UTC()
	fileName := filepath.Base(filePath)
	parentDir := filepath.Dir(filePath)
	createdFile, err := db.conn.Exec(
		`INSERT INTO watched_files (watcher_name, file_path, file_name, flushed, detected_at) VALUES (?,?,?,?,?)`,
		watcherName, parentDir, fileName, flushed, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("add watched file: %w", err)
	}
	id, err := createdFile.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get created id: %w", err)
	}
	return &WatchedFile{ID: id, WatcherName: watcherName, FilePath: parentDir, DetectedAt: now, Flushed: flushed}, nil
}

// RemoveWatchedFile deletes the entry for a given watcher and full file path.
func (db *DB) RemoveWatchedFile(watcherName string, filePath string) error {
	fileName := filepath.Base(filePath)
	parentDir := filepath.Dir(filePath)
	_, err := db.conn.Exec(
		`DELETE FROM watched_files WHERE watcher_name=? AND file_path=? AND file_name=?`,
		watcherName, parentDir, fileName,
	)
	return err
}

// ListWatchedFiles returns all unflushed watched files for the given watcher.
func (db *DB) ListWatchedFiles(watcherName string) ([]*WatchedFile, error) {
	rows, err := db.conn.Query(
		`SELECT id, watcher_name, file_path, file_name, flushed, detected_at
		 FROM watched_files WHERE watcher_name = ?`, watcherName)
	if err != nil {
		return nil, fmt.Errorf("list watched files: %w", err)
	}
	defer rows.Close()
	var result []*WatchedFile
	for rows.Next() {
		var wf WatchedFile
		var flushedInt int
		var detectedStr string
		var fileName string
		if err := rows.Scan(&wf.ID, &wf.WatcherName, &wf.FilePath, &fileName, &flushedInt, &detectedStr); err != nil {
			return nil, fmt.Errorf("scan watched file: %w", err)
		}
		wf.FilePath = filepath.Join(wf.FilePath, fileName)
		wf.Flushed = flushedInt == 1
		wf.DetectedAt, _ = time.Parse(time.RFC3339, detectedStr)
		result = append(result, &wf)
	}
	return result, rows.Err()
}

func (db *DB) FlushWatchedFiles(ids []int64) error {
	if len(ids) == 0 {
		slog.Warn("no files to flush")
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "UPDATE watched_files SET flushed=1 WHERE id IN (" + strings.Join(placeholders, ",") + ")"

	_, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("flush watched files: %w", err)
	}
	return nil
}

func (db *DB) ListPendingFlushes(name string) ([]*PendingFlush, error) {
	rows, err := db.conn.Query(
		`SELECT watched_file_id, watcher_name, file_path, file_name, target_path
			FROM pending_file_flushes WHERE watcher_name = ?`, name)
	if err != nil {
		return nil, fmt.Errorf("list pending flushes: %w", err)
	}
	defer rows.Close()
	var result []*PendingFlush
	for rows.Next() {
		var pf PendingFlush
		if err := rows.Scan(&pf.WatchedFileID, &pf.WatcherName, &pf.FilePath, &pf.FileName, &pf.TargetPath); err != nil {
			return nil, fmt.Errorf("scan pending flush: %w", err)
		}
		result = append(result, &pf)
	}
	return result, rows.Err()
}
