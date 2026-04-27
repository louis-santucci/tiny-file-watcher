package database

type RedirectionRepository interface {
	AddRedirection(watcherName string, targetPath string, autoFlush bool, targetMachineName string) (*FileRedirection, error)
	GetRedirection(watcherName string) (*FileRedirection, error)
	RemoveRedirection(watcherName string) error
	UpdateRedirection(watcherName string, filePath *string, autoFlush *bool) (*FileRedirection, error)
}
