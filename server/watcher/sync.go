package watcher

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	. "tiny-file-watcher/internal"
	"tiny-file-watcher/server/database"
	tfwssh "tiny-file-watcher/server/ssh"

	"github.com/kr/fs"
	"github.com/pkg/sftp"
)

// RemoteFS abstracts the file-system operations performed against the remote
// machine during a sync.  In production the SFTP-backed implementation is
// used; in tests a local-filesystem implementation can be injected instead.
type RemoteFS interface {
	FileOpener
	// Walk returns a Walker rooted at the given path.
	Walk(path string) *fs.Walker
}

// sftpRemoteFS adapts *sftp.Client to the RemoteFS interface.
type sftpRemoteFS struct{ c *sftp.Client }

func (s sftpRemoteFS) OpenFile(path string, f int) (io.ReadCloser, error) {
	return s.c.OpenFile(path, f)
}

func (s sftpRemoteFS) Walk(path string) *fs.Walker {
	return s.c.Walk(path)
}

// localRemoteFS implements RemoteFS against the local filesystem.
// It is used by unit tests to avoid dialling SSH.
type localRemoteFS struct{}

// LocalRemoteFS returns a RemoteFS that uses the local filesystem.
// Intended for use in tests.
func LocalRemoteFS() RemoteFS { return localRemoteFS{} }

func (localRemoteFS) OpenFile(path string, _ int) (io.ReadCloser, error) {
	return os.OpenFile(path, os.O_RDONLY, 0)
}

func (localRemoteFS) Walk(path string) *fs.Walker {
	return fs.Walk(path)
}

type SyncJob struct {
	watcher           *database.FileWatcher
	machine           *database.Machine
	logger            *slog.Logger
	fileRepository    database.FileRepository
	watcherRepository database.FileWatcherRepository
	transactor        database.Transactor
	// remoteFS overrides the SSH/SFTP transport when set (used in tests).
	remoteFS RemoteFS
	// onLog is an optional callback invoked with a human-readable message at
	// each major step of the sync process.  Used by StreamSyncWatcher to send
	// LOG events to the client.  When nil, calls are silently ignored.
	onLog func(string)
}

// SyncJobOption is a functional option for SyncJob.
type SyncJobOption func(*SyncJob)

// WithRemoteFS injects a custom RemoteFS implementation into the SyncJob,
// bypassing the SSH/SFTP dial.  Intended for use in tests.
func WithRemoteFS(rfs RemoteFS) SyncJobOption {
	return func(j *SyncJob) {
		j.remoteFS = rfs
	}
}

// WithLogCallback sets a callback that is invoked with a human-readable
// progress message at each major step of the sync process.  Used by
// StreamSyncWatcher to forward LOG events to the streaming client.
func WithLogCallback(fn func(string)) SyncJobOption {
	return func(j *SyncJob) {
		j.onLog = fn
	}
}

// log invokes the onLog callback when one has been set.
func (j *SyncJob) log(msg string) {
	if j.onLog != nil {
		j.onLog(msg)
	}
}

type SyncResult struct {
	AddedCount   int32
	RemovedCount int32
	AddedFiles   *Set[string]
	RemovedFiles []string
}

func NewSyncJob(logger *slog.Logger, watcher *database.FileWatcher, machine *database.Machine, fileRepo database.FileRepository, watcherRepo database.FileWatcherRepository, transactor database.Transactor, opts ...SyncJobOption) *SyncJob {
	j := &SyncJob{
		watcher:           watcher,
		machine:           machine,
		logger:            logger,
		fileRepository:    fileRepo,
		watcherRepository: watcherRepo,
		transactor:        transactor,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

func (j *SyncJob) Run(flush bool) (*SyncResult, error) {
	j.logger.Info("starting sync job")

	rfs, err := j.openRemoteFS()
	if err != nil {
		return nil, err
	}

	j.log("loading watched files from database")
	watchedFiles, err := j.fileRepository.ListWatchedFiles(j.watcher.Name)
	if err != nil {
		j.logger.Error("failed to list watched files", "error", err)
		return nil, err
	}
	watchedFilesSet := NewSetWithSize[string](len(watchedFiles))
	for _, watchedFile := range watchedFiles {
		watchedFilesSet.Add(watchedFile.FilePath)
	}

	j.log("loading ignore rules")
	ignorer, err := LoadIgnore(rfs, j.watcher.SourcePath+"/"+ignoreFileName, j.logger)
	if err != nil {
		j.logger.Error("sync: error loading .tfwignore", "watcher", j.watcher.Name, "path", j.watcher.SourcePath, "err", err)
		j.log("ignoring rules")
		ignorer = noopIgnorer{}
	}

	// using batch of results, check in db if file exists for this file watcher, if not, create it, if yes, do nothing

	j.log("walking source path")
	onDisk, addedFiles, err := j.handleCurrentPaths(rfs, watchedFilesSet, ignorer)
	if err != nil {
		j.logger.Error("sync: error handling current paths", "error", err, "watcher", j.watcher.Name)
		return nil, err
	}

	j.log("computing removed files")
	removedFiles := make([]string, 0)
	for _, watchedFile := range watchedFiles {
		if !onDisk.Contains(watchedFile.FilePath) {
			j.logger.Debug("sync: removing watched file that no longer exists on disk", "path", watchedFile.FilePath, "watcher", j.watcher.Name)
			removedFiles = append(removedFiles, watchedFile.FilePath)
		}
	}

	// bulk insert new files
	// bulk remove deleted files
	j.log("saving updates to database")
	err = j.saveUpdates(addedFiles, removedFiles, flush)
	if err != nil {
		j.logger.Error("sync: error saving updates to database", "error", err, "watcher", j.watcher.Name)
		return nil, err
	}

	results := &SyncResult{
		AddedCount:   int32(addedFiles.Size()),
		RemovedCount: int32(len(removedFiles)),
		AddedFiles:   addedFiles,
		RemovedFiles: removedFiles,
	}

	j.logger.Info("sync job finished", "added_count", results.AddedCount, "removed_count", results.RemovedCount, "watcher", j.watcher.Name)

	return results, nil
}

// openRemoteFS returns the RemoteFS to use for this sync run.
// If one was injected via WithRemoteFS it is returned directly.
// Otherwise an SSH/SFTP connection is established using the credentials
// stored on the machine record.
func (j *SyncJob) openRemoteFS() (RemoteFS, error) {
	if j.remoteFS != nil {
		return j.remoteFS, nil
	}

	j.logger.Debug("private key path", "path", j.machine.SSHPrivateKeyPath)

	cfg := tfwssh.MachineConfig{
		Name:              j.machine.Name,
		IP:                j.machine.IP,
		SSHPort:           j.machine.SSHPort,
		SSHUser:           j.machine.SSHUser,
		SSHPrivateKeyPath: j.machine.SSHPrivateKeyPath,
	}

	j.logger.Debug("sync: SSH URL: " + cfg.IP + ":" + strconv.Itoa(int(cfg.SSHPort)))

	client, err := tfwssh.NewClient(cfg)
	if err != nil {
		j.logger.Error("failed to connect to machine", "error", err)
		return nil, err
	}
	return sftpRemoteFS{c: client.SFTPClient}, nil
}

func (j *SyncJob) saveUpdates(addedFiles *Set[string], removedFiles []string, flush bool) error {
	err := j.transactor.WithTransaction(context.Background(), func(repo database.TransactionalFileRepository) error {
		if addedFiles.Size() > 0 {
			_, err := repo.BulkAddWatchedFiles(j.watcher.Name, addedFiles, flush)
			if err != nil {
				return err
			}
		}
		if len(removedFiles) > 0 {
			err := repo.BulkRemoveWatchedFiles(j.watcher.Name, removedFiles)
			if err != nil {
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

func (j *SyncJob) handleCurrentPaths(rfs RemoteFS, watchedFilesSet *Set[string], ignorer Ignorer) (*Set[string], *Set[string], error) {
	onDisk := NewSet[string]()
	addedFiles := NewSet[string]()

	// walk the source path and check if the file exists in the db for this watcher, if not, create it, if yes, do nothing
	queue := []*fs.Walker{rfs.Walk(j.watcher.SourcePath)}
	analyzed := NewSet[string]()
	analyzed.Add(j.watcher.SourcePath)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		newWalkers := j.drainWalker(current, rfs, analyzed, watchedFilesSet, ignorer, onDisk, addedFiles)
		queue = append(queue, newWalkers...)
	}

	return onDisk, addedFiles, nil
}

// drainWalker steps through a single Walker, delegating each entry to either
// enqueueSubdir (directories) or processFile (regular files).
// It returns any new sub-walkers that should be added to the outer queue.
func (j *SyncJob) drainWalker(walker *fs.Walker, rfs RemoteFS, analyzed *Set[string], watchedFilesSet *Set[string], ignorer Ignorer, onDisk *Set[string], addedFiles *Set[string]) []*fs.Walker {
	var newWalkers []*fs.Walker
	for walker.Step() {
		if walker.Err() != nil {
			j.logger.Error("sync: error walking source path", "error", walker.Err(), "watcher", j.watcher.Name, "path", walker.Path())
			continue
		}
		if walker.Stat().IsDir() {
			if w := j.enqueueSubdir(walker.Path(), rfs, analyzed); w != nil {
				newWalkers = append(newWalkers, w)
			}
			continue
		}
		j.processFile(walker.Path(), walker.Stat().Name(), watchedFilesSet, ignorer, onDisk, addedFiles)
	}
	return newWalkers
}

// enqueueSubdir returns a new Walker for path if it hasn't been analyzed yet,
// or nil if it should be skipped.
func (j *SyncJob) enqueueSubdir(path string, rfs RemoteFS, analyzed *Set[string]) *fs.Walker {
	if analyzed.Contains(path) {
		return nil
	}
	j.logger.Debug("sync: adding subdirectory", "path", path, "watcher", j.watcher.Name)
	analyzed.Add(path)
	return rfs.Walk(path)
}

// processFile records a single file entry: skips ignored files, adds non-tracked
// files to addedFiles, and always registers present files in onDisk.
func (j *SyncJob) processFile(path, name string, watchedFilesSet *Set[string], ignorer Ignorer, onDisk *Set[string], addedFiles *Set[string]) {
	if ignorer.MatchesPath(j.watcher.SourcePath, path) || name == ignoreFileName {
		j.logger.Debug("sync: skipping ignored file (.tfwignore rule)", "path", path, "watcher", j.watcher.Name)
		return
	}
	onDisk.Add(path)
	if watchedFilesSet.Contains(path) {
		return
	}
	j.logger.Debug("sync: adding new watched file", "path", path)
	addedFiles.Add(path)
}
