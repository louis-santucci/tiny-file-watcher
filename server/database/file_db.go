package database

import (
	"fmt"
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
	_, err := db.conn.Exec(`UPDATE watched_files SET flushed=1 WHERE watcher_id in ?`, ids)
	if err != nil {
		return fmt.Errorf("flush watched files: %w", err)
	}
	return nil
}

func extractFileName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
