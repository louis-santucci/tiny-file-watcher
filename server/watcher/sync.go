package watcher

import (
	"context"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	. "tiny-file-watcher/internal"
	"tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"

	"github.com/kr/fs"
	"golang.org/x/crypto/ssh"
)

// SyncJob orchestrates a single reconciliation run for one watcher:
// it walks the remote source directory, compares the results against the
// database, and persists any additions or removals in a single transaction.
type SyncJob struct {
	watcher           *database.FileWatcher
	machine           *database.Machine
	logger            *slog.Logger
	sshConfig         *config.SSHConfig
	publicKey         ssh.PublicKey
	fileRepository    database.FileRepository
	watcherRepository database.FileWatcherRepository
	transactor        database.Transactor
	// remoteFS overrides the SSH/SFTP transport when set (used in tests).
	remoteFS RemoteFS
}

// SyncJobOption is a functional option for SyncJob.
type SyncJobOption func(*SyncJob)

// WithRemoteFS injects a custom RemoteFS implementation into the SyncJob,
// bypassing the SSH/SFTP dial. Intended for use in tests.
func WithRemoteFS(rfs RemoteFS) SyncJobOption {
	return func(j *SyncJob) {
		j.remoteFS = rfs
	}
}

// SyncResult summarises the outcome of a single sync run.
type SyncResult struct {
	AddedCount   int32
	RemovedCount int32
	AddedFiles   []string
	RemovedFiles []string
}

// NewSyncJob constructs a SyncJob with the provided dependencies.
func NewSyncJob(
	logger *slog.Logger,
	watcher *database.FileWatcher,
	machine *database.Machine,
	sshConfig *config.SSHConfig,
	publicKey ssh.PublicKey,
	fileRepo database.FileRepository,
	watcherRepo database.FileWatcherRepository,
	transactor database.Transactor,
	opts ...SyncJobOption,
) *SyncJob {
	j := &SyncJob{
		watcher:           watcher,
		machine:           machine,
		logger:            logger,
		sshConfig:         sshConfig,
		publicKey:         publicKey,
		fileRepository:    fileRepo,
		watcherRepository: watcherRepo,
		transactor:        transactor,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

// Run executes the sync job. When flush is true newly detected files are
// immediately marked for flushing in the database.
func (j *SyncJob) Run(flush bool) (*SyncResult, error) {
	j.logger.Info("starting sync job")

	rfs, err := j.resolveRemoteFS()
	if err != nil {
		return nil, err
	}

	watchedFiles, err := j.fileRepository.ListWatchedFiles(j.watcher.Name)
	if err != nil {
		j.logger.Error("failed to list watched files", "error", err)
		return nil, err
	}

	watchedFilesSet := buildWatchedFilesSet(watchedFiles)

	ignorer, err := LoadIgnore(rfs, j.watcher.SourcePath+"/"+ignoreFileName, j.logger)
	if err != nil {
		j.logger.Error("sync: error loading .tfwignore", "watcher", j.watcher.Name, "path", j.watcher.SourcePath, "err", err)
		ignorer = noopIgnorer{}
	}

	onDisk, addedFiles, err := j.walkSourcePath(rfs, watchedFilesSet, ignorer)
	if err != nil {
		j.logger.Error("sync: error walking source path", "error", err, "watcher", j.watcher.Name)
		return nil, err
	}

	removedFiles := j.detectRemovals(watchedFiles, onDisk)

	if err = j.saveUpdates(addedFiles, removedFiles, flush); err != nil {
		j.logger.Error("sync: error saving updates to database", "error", err, "watcher", j.watcher.Name)
		return nil, err
	}

	results := &SyncResult{
		AddedCount:   int32(len(addedFiles)),
		RemovedCount: int32(len(removedFiles)),
		AddedFiles:   slices.Collect(maps.Values(addedFiles)),
		RemovedFiles: removedFiles,
	}

	j.logger.Info("sync job finished",
		"added_count", results.AddedCount,
		"removed_count", results.RemovedCount,
		"watcher", j.watcher.Name,
	)

	return results, nil
}

// resolveRemoteFS returns the injected RemoteFS when available, otherwise it
// establishes a live SSH/SFTP connection.
func (j *SyncJob) resolveRemoteFS() (RemoteFS, error) {
	if j.remoteFS != nil {
		return j.remoteFS, nil
	}
	return dialSFTP(j.logger, j.machine, j.sshConfig)
}

// buildWatchedFilesSet converts the slice of watched files returned by the
// repository into a set of file paths for O(1) lookup.
func buildWatchedFilesSet(watchedFiles []*database.WatchedFile) *Set[string] {
	s := NewSetWithSize[string](len(watchedFiles))
	for _, f := range watchedFiles {
		s.Add(f.FilePath)
	}
	return s
}

// detectRemovals returns the paths that are tracked in the database but are no
// longer present on disk.
func (j *SyncJob) detectRemovals(watchedFiles []*database.WatchedFile, onDisk *Set[string]) []string {
	removed := make([]string, 0)
	for _, f := range watchedFiles {
		if !onDisk.Contains(f.FilePath) {
			j.logger.Debug("sync: removing watched file that no longer exists on disk",
				"path", f.FilePath,
				"watcher", j.watcher.Name,
			)
			removed = append(removed, f.FilePath)
		}
	}
	return removed
}

// saveUpdates persists additions and removals inside a single database
// transaction.
func (j *SyncJob) saveUpdates(addedFiles map[string]string, removedFiles []string, flush bool) error {
	err := j.transactor.WithTransaction(context.Background(), func(repo database.TransactionalFileRepository) error {
		if len(addedFiles) > 0 {
			if _, err := repo.BulkAddWatchedFiles(j.watcher.Name, addedFiles, flush); err != nil {
				return err
			}
		}
		if len(removedFiles) > 0 {
			if err := repo.BulkRemoveWatchedFiles(j.watcher.Name, removedFiles); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		j.logger.Error("sync: database error on update", "error", err, "watcher", j.watcher.Name)
		return err
	}
	return nil
}

// walkSourcePath recursively walks the watcher's source directory on the remote
// filesystem. It returns:
//   - onDisk: set of every file path currently present under the source directory
//   - addedFiles: map of filename → full path for files not yet tracked in the DB
func (j *SyncJob) walkSourcePath(rfs RemoteFS, watchedFilesSet *Set[string], ignorer Ignorer) (*Set[string], map[string]string, error) {
	onDisk := NewSet[string]()
	addedFiles := make(map[string]string)

	// BFS over the directory tree using a walker queue so that subdirectories
	// discovered during the walk are also fully traversed.
	queue := []*fs.Walker{rfs.Walk(j.watcher.SourcePath)}
	visited := NewSet[string]()
	visited.Add(j.watcher.SourcePath)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for current.Step() {
			if err := current.Err(); err != nil {
				j.logger.Error("sync: error walking source path",
					"error", err,
					"watcher", j.watcher.Name,
					"path", current.Path(),
				)
				continue
			}

			if current.Stat().IsDir() {
				j.handleDirectory(current, queue, visited)
				continue
			}

			// Skip files matched by .tfwignore or the ignore file itself.
			if current.Stat().Name() == ignoreFileName || ignorer.MatchesPath(j.watcher.SourcePath, current.Path()) {
				j.logger.Debug("sync: skipping ignored file", "path", current.Path(), "watcher", j.watcher.Name)
				continue
			}

			if !watchedFilesSet.Contains(current.Path()) {
				j.logger.Debug("sync: new file detected", "path", current.Path())
				addedFiles[filepath.Base(current.Path())] = current.Path()
			}
			onDisk.Add(current.Path())
		}
	}

	return onDisk, addedFiles, nil
}

func (j *SyncJob) handleDirectory(current *fs.Walker, queue []*fs.Walker, visited *Set[string]) {
	if !visited.Contains(current.Path()) {
		j.logger.Debug("sync: entering subdirectory", "path", current.Path(), "watcher", j.watcher.Name)
		queue = append(queue, j.remoteFS.Walk(current.Path()))
		visited.Add(current.Path())
	}
}
