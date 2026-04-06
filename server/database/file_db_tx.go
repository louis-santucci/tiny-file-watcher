package database

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// TxDB wraps a *sql.Tx and implements watcher.FileRepository within a transaction.
type TxDB struct {
	tx *sql.Tx
}

func (t *TxDB) BulkAddWatchedFiles(watcherName string, files map[string]string, flushed bool) ([]*WatchedFile, error) {
	if len(files) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	placeholders := make([]string, 0, len(files))
	args := make([]interface{}, 0, len(files)*5)

	type entry struct {
		name      string
		parentDir string
	}
	entries := make([]entry, 0, len(files))

	for name, path := range files {
		parentDir := filepath.Dir(path)
		placeholders = append(placeholders, "(?,?,?,?,?)")
		args = append(args, watcherName, parentDir, name, flushed, nowStr)
		entries = append(entries, entry{name: name, parentDir: parentDir})
	}

	query := "INSERT INTO watched_files (watcher_name, file_path, file_name, flushed, detected_at) VALUES " +
		strings.Join(placeholders, ",")

	result, err := t.tx.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("tx bulk add watched files: %w", err)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("tx bulk add watched files: get last insert id: %w", err)
	}

	n := int64(len(entries))
	firstID := lastID - n + 1

	watchedFiles := make([]*WatchedFile, len(entries))
	for i, e := range entries {
		watchedFiles[i] = &WatchedFile{
			ID:          firstID + int64(i),
			WatcherName: watcherName,
			FilePath:    filepath.Join(e.parentDir, e.name),
			Flushed:     flushed,
			DetectedAt:  now,
		}
	}
	return watchedFiles, nil
}

func (t *TxDB) BulkRemoveWatchedFiles(watcherName string, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	conditions := make([]string, len(filePaths))
	args := make([]interface{}, 0, 1+len(filePaths)*2)
	args = append(args, watcherName)

	for i, fp := range filePaths {
		conditions[i] = "(file_path=? AND file_name=?)"
		args = append(args, filepath.Dir(fp), filepath.Base(fp))
	}

	query := "DELETE FROM watched_files WHERE watcher_name=? AND (" + strings.Join(conditions, " OR ") + ")"

	_, err := t.tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("tx bulk remove watched files: %w", err)
	}
	return nil
}
