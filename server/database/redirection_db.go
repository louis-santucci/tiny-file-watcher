package database

import "time"

type FileRedirection struct {
	ID         int64
	WatcherID  int64
	TargetPath string
	AutoFlush  bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
