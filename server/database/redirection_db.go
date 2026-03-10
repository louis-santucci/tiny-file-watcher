package database

import (
	"fmt"
	"strings"
	"time"
)

type FileRedirection struct {
	WatcherName string
	TargetPath  string
	AutoFlush   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (db *DB) AddRedirection(watcherName string, filePath string, autoFlush bool) (*FileRedirection, error) {
	now := time.Now().UTC()
	_, err := db.conn.Exec("INSERT INTO file_redirections (watcher_name, target_path, auto_flush, created_at, updated_at) VALUES (?,?,?,?,?)", watcherName, filePath, autoFlush, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	return &FileRedirection{WatcherName: watcherName, TargetPath: filePath, AutoFlush: autoFlush, CreatedAt: now, UpdatedAt: now}, nil
}

func (db *DB) GetRedirection(watcherName string) (*FileRedirection, error) {
	row := db.conn.QueryRow("SELECT watcher_name, target_path, auto_flush, created_at, updated_at FROM file_redirections WHERE watcher_name = ?", watcherName)
	return scanFileRedirection(row)
}

func (db *DB) RemoveRedirection(watcherName string) error {
	res, err := db.conn.Exec("DELETE FROM file_redirections WHERE watcher_name = ?", watcherName)
	if err != nil {
		return fmt.Errorf("delete redirection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("redirection for watcher %s not found", watcherName)
	}
	return nil
}

func (db *DB) UpdateRedirection(watcherName string, filePath *string, autoFlush *bool) (*FileRedirection, error) {
	if filePath == nil && autoFlush == nil {
		return db.GetRedirection(watcherName)
	}

	now := time.Now().UTC()
	setClauses := []string{}
	args := []any{}

	if filePath != nil {
		setClauses = append(setClauses, "target_path = ?")
		args = append(args, filePath)
	}
	if autoFlush != nil {
		setClauses = append(setClauses, "auto_flush = ?")
		args = append(args, autoFlush)
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now.Format(time.RFC3339))
	args = append(args, watcherName)

	query := fmt.Sprintf("UPDATE file_redirections SET %s WHERE watcher_name = ?", strings.Join(setClauses, ", "))
	res, err := db.conn.Exec(query, args...)
	if err != nil {
		return nil, fmt.Errorf("update redirection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("redirection for watcher %s not found", watcherName)
	}
	return db.GetRedirection(watcherName)
}

func scanFileRedirection(s scanner) (*FileRedirection, error) {
	var fileRedirection FileRedirection
	if err := s.Scan(&fileRedirection.WatcherName, &fileRedirection.TargetPath, &fileRedirection.AutoFlush, &fileRedirection.CreatedAt, &fileRedirection.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan file redirection: %w", err)
	}
	return &fileRedirection, nil
}
