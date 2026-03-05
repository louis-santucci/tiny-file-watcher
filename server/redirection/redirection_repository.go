package redirection

import "tiny-file-watcher/server/database"

type RedirectionRepository interface {
	AddRedirection(watcherID int64, filePath string, autoFlush bool) (*database.FileRedirection, error)
	GetRedirectionByID(watcherID int64, filePath string) (*database.FileRedirection, error)
	RemoveRedirection(watcherID int64) error
	UpdateRedirection(watcherID int64, filePath string, autoFlush bool) (*database.FileRedirection, error)
}
