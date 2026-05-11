package database

type RedirectionRepository interface {
	AddRedirection(watcherName string, targetPath string, targetMachineName string) (*FileRedirection, error)
	GetRedirection(watcherName string) (*FileRedirection, error)
	RemoveRedirection(watcherName string) error
	UpdateRedirection(watcherName string, filePath *string) (*FileRedirection, error)
}
