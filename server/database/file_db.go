package database

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
	"tiny-file-watcher/internal"
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
	// Source machine SSH details
	MachineName              string
	MachineIP                string
	MachineSSHPort           int32
	MachineSSHUser           string
	MachineSSHPrivateKeyPath string
	// Target machine SSH details
	TargetMachineName              string
	TargetMachineIP                string
	TargetMachineSSHPort           int32
	TargetMachineSSHUser           string
	TargetMachineSSHPrivateKeyPath string
}

func (db *DB) WithTransaction(ctx context.Context, fn func(tx TransactionalFileRepository) error) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txDB := &TxDB{tx: tx}
	if err := fn(txDB); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
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

// BulkAddWatchedFiles inserts multiple watched files in a single INSERT statement.
// files is a map of file name to full file path. Returns early with a warning if the map is empty.
func (db *DB) BulkAddWatchedFiles(watcherName string, files *internal.Set[string], flushed bool) ([]*WatchedFile, error) {
	if files.Size() == 0 {
		slog.Warn("no files to bulk insert")
		return nil, nil
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	placeholders := make([]string, 0, files.Size())
	args := make([]interface{}, 0, files.Size()*5)

	// Collect entries to rebuild results after insert (map iteration order is fine; callers use ElementsMatch).
	type entry struct {
		name      string
		parentDir string
	}
	entries := make([]entry, 0, files.Size())

	fileItems := files.Items()

	for _, path := range fileItems {
		parentDir := filepath.Dir(path)
		name := filepath.Base(path)
		placeholders = append(placeholders, "(?,?,?,?,?)")
		args = append(args, watcherName, parentDir, name, flushed, nowStr)
		entries = append(entries, entry{name: name, parentDir: parentDir})
	}

	query := "INSERT INTO watched_files (watcher_name, file_path, file_name, flushed, detected_at) VALUES " +
		strings.Join(placeholders, ",")

	result, err := db.conn.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("bulk add watched files: %w", err)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("bulk add watched files: get last insert id: %w", err)
	}

	// SQLite assigns sequential IDs for a multi-row INSERT; the first inserted row
	// gets lastID - N + 1, the last gets lastID, in the same order as the VALUES list.
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

// BulkRemoveWatchedFiles deletes multiple watched files for a given watcher in a single DELETE statement.
// filePaths are full file paths. Returns early with a warning if the slice is empty.
func (db *DB) BulkRemoveWatchedFiles(watcherName string, filePaths []string) error {
	if len(filePaths) == 0 {
		slog.Warn("no files to bulk delete")
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

	_, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("bulk remove watched files: %w", err)
	}
	return nil
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
		`SELECT watched_file_id, watcher_name, file_path, file_name, target_path,
		        machine_name, machine_ip, machine_ssh_port, machine_ssh_user, machine_ssh_private_key_path,
		        target_machine_name, target_machine_ip, target_machine_ssh_port, target_machine_ssh_user, target_machine_ssh_private_key_path
			FROM pending_file_flushes WHERE watcher_name = ?`, name)
	if err != nil {
		return nil, fmt.Errorf("list pending flushes: %w", err)
	}
	defer rows.Close()
	var result []*PendingFlush
	for rows.Next() {
		var pf PendingFlush
		if err := rows.Scan(
			&pf.WatchedFileID, &pf.WatcherName, &pf.FilePath, &pf.FileName, &pf.TargetPath,
			&pf.MachineName, &pf.MachineIP, &pf.MachineSSHPort, &pf.MachineSSHUser, &pf.MachineSSHPrivateKeyPath,
			&pf.TargetMachineName, &pf.TargetMachineIP, &pf.TargetMachineSSHPort, &pf.TargetMachineSSHUser, &pf.TargetMachineSSHPrivateKeyPath,
		); err != nil {
			return nil, fmt.Errorf("scan pending flush: %w", err)
		}
		result = append(result, &pf)
	}
	return result, rows.Err()
}
