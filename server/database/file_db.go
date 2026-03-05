package database

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// WatchedFile mirrors the watched_files table row.
type WatchedFile struct {
	ID         int64
	WatcherID  int64
	FilePath   string
	Flushed    bool
	DetectedAt time.Time
}

// PendingFlush mirrors one row of the pending_file_flushes view.
type PendingFlush struct {
	WatchedFileID int64
	WatcherID     int64
	WatcherName   string
	FilePath      string
	FileName      string
	TargetPath    string
}

// AddWatchedFile inserts a newly detected file.
func (db *DB) AddWatchedFile(watcherID int64, filePath string, flushed bool) (*WatchedFile, error) {
	now := time.Now().UTC()
	// extract the file name from the path for easier querying later.
	fileName := extractFileName(filePath)
	createdFile, err := db.conn.Exec(
		`INSERT INTO watched_files (watcher_id, file_path, file_name, flushed, detected_at) VALUES (?,?,?,?,?)`,
		watcherID, filePath, fileName, flushed, now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("add watched file: %w", err)
	}
	id, err := createdFile.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get created id: %w", err)
	}
	return &WatchedFile{ID: id, WatcherID: watcherID, FilePath: filePath, DetectedAt: now, Flushed: flushed}, nil
}

// RemoveWatchedFile deletes the entry for a given watcher and file path.
func (db *DB) RemoveWatchedFile(watcherID int64, filePath string) error {
	_, err := db.conn.Exec(`DELETE FROM watched_files WHERE watcher_id=? AND file_path=?`, watcherID, filePath)
	return err
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
		`SELECT watched_file_id, watcher_id, watcher_name, file_path, file_name, target_path
			FROM pending_file_flushes WHERE watcher_name = ?`, name)
	if err != nil {
		return nil, fmt.Errorf("list pending flushes: %w", err)
	}
	defer rows.Close()
	var result []*PendingFlush
	for rows.Next() {
		var pf PendingFlush
		if err := rows.Scan(&pf.WatchedFileID, &pf.WatcherID, &pf.WatcherName, &pf.FilePath, &pf.FileName, &pf.TargetPath); err != nil {
			return nil, fmt.Errorf("scan pending flush: %w", err)
		}
		result = append(result, &pf)
	}
	return result, rows.Err()
}

func extractFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
