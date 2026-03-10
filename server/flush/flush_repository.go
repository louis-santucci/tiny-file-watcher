package flush

import "tiny-file-watcher/server/database"

// FlushRepository defines the persistence operations required by FlushService.
type FlushRepository interface {
	ListPendingFlushes(watcherName string) ([]*database.PendingFlush, error)
	FlushWatchedFiles(ids []int64) error
}

// Compile-time assertion: *database.DB must satisfy FlushRepository.
var _ FlushRepository = (*database.DB)(nil)
