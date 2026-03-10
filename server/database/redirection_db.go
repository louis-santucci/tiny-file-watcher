package database

import "time"

type FileRedirection struct {
	WatcherName string
	TargetPath  string
	AutoFlush   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
