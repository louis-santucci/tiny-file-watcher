package redirection

import "tiny-file-watcher/server/database"

type RedirectionRepository interface {
	AddRedirection(watcherName string, targetPath string, autoFlush bool) (*database.FileRedirection, error)
	GetRedirection(watcherName string) (*database.FileRedirection, error)
	RemoveRedirection(watcherName string) error
	UpdateRedirection(watcherName string, filePath *string, autoFlush *bool) (*database.FileRedirection, error)
}
