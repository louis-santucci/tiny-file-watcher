package watcher

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	. "tiny-file-watcher/internal"
	"tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"

	"github.com/kr/fs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
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
	sshConfig         *config.SSHConfig
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
	AddedFiles   []string
	RemovedFiles []string
}

func NewSyncJob(logger *slog.Logger, watcher *database.FileWatcher, machine *database.Machine, sshConfig *config.SSHConfig, fileRepo database.FileRepository, watcherRepo database.FileWatcherRepository, transactor database.Transactor, opts ...SyncJobOption) *SyncJob {
	j := &SyncJob{
		watcher:           watcher,
		machine:           machine,
		logger:            logger,
		sshConfig:         sshConfig,
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
	err = j.saveUpdates(*addedFiles, removedFiles, flush)
	if err != nil {
		j.logger.Error("sync: error saving updates to database", "error", err, "watcher", j.watcher.Name)
		return nil, err
	}

	results := &SyncResult{
		AddedCount:   int32(len(*addedFiles)),
		RemovedCount: int32(len(removedFiles)),
		AddedFiles:   slices.Collect(maps.Values(*addedFiles)),
		RemovedFiles: removedFiles,
	}

	j.logger.Info("sync job finished", "added_count", results.AddedCount, "removed_count", results.RemovedCount, "watcher", j.watcher.Name)

	return results, nil
}

// openRemoteFS returns the RemoteFS to use for this sync run.
// If one was injected via WithRemoteFS it is returned directly.
// Otherwise an SSH/SFTP connection is established using a FixedHostKey
// loaded from the path stored on the machine record.
func (j *SyncJob) openRemoteFS() (RemoteFS, error) {
	if j.remoteFS != nil {
		return j.remoteFS, nil
	}

	j.logger.Debug("private key path", "path", filepath.Join(j.sshConfig.PrivateKeysPath, j.machine.SSHKeyName))

	// Load the host's public key and build a FixedHostKey callback.
	hostKeyBytes, err := os.ReadFile(j.machine.SSHHostPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read host public key %q: %w", j.machine.SSHHostPublicKeyPath, err)
	}
	hostPubKey, _, _, _, err := ssh.ParseAuthorizedKey(hostKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse host public key %q: %w", j.machine.SSHHostPublicKeyPath, err)
	}

	sshConfig := ssh.ClientConfig{
		User: j.machine.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				keyPath := filepath.Join(j.sshConfig.PrivateKeysPath, j.machine.SSHKeyName)
				keyBytes, err := os.ReadFile(keyPath)
				if err != nil {
					return nil, err
				}
				key, err := ssh.ParsePrivateKey(keyBytes)
				if err != nil {
					return nil, err
				}
				return []ssh.Signer{key}, nil
			}),
		},
		HostKeyCallback: ssh.FixedHostKey(hostPubKey),
	}

	sshUrl := j.machine.IP + ":" + strconv.Itoa(int(j.machine.SSHPort))
	j.logger.Debug("sync: SSH URL: " + sshUrl)
	sshConnection, err := ssh.Dial("tcp", sshUrl, &sshConfig)
	if err != nil {
		j.logger.Error("failed to connect to machine", "error", err)
		return nil, err
	}
	sftpClient, err := sftp.NewClient(sshConnection)
	if err != nil {
		j.logger.Error("failed to create SFTP sftpClient", "error", err)
		sshConnection.Close()
		return nil, err
	}
	return sftpRemoteFS{c: sftpClient}, nil
}

func (j *SyncJob) saveUpdates(addedFiles map[string]string, removedFiles []string, flush bool) error {
	err := j.transactor.WithTransaction(context.Background(), func(repo database.TransactionalFileRepository) error {
		if len(addedFiles) > 0 {
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

func (j *SyncJob) handleCurrentPaths(rfs RemoteFS, watchedFilesSet *Set[string], ignorer Ignorer) (*Set[string], *map[string]string, error) {
	onDisk := NewSet[string]()
	addedFiles := make(map[string]string)

	// walk the source path and check if the file exists in the db for this watcher, if not, create it, if yes, do nothing
	queue := []*fs.Walker{rfs.Walk(j.watcher.SourcePath)}
	analyzed := NewSet[string]()
	analyzed.Add(j.watcher.SourcePath)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for current.Step() {
			if current.Err() != nil {
				j.logger.Error("sync: error walking source path", "error", current.Err(), "watcher", j.watcher.Name, "path", current.Path())
				continue
			}
			if current.Stat().IsDir() {
				if !analyzed.Contains(current.Path()) {
					j.logger.Debug("sync: adding subdirectory", "path", current.Path(), "watcher", j.watcher.Name)
					queue = append(queue, rfs.Walk(current.Path())) // enqueue subdirectory
					analyzed.Add(current.Path())
				}
				continue
			}
			if ignorer.MatchesPath(j.watcher.SourcePath, current.Path()) || current.Stat().Name() == ".tfwignore" {
				j.logger.Debug("sync: skipping ignored file (.tfwignore rule)", "path", current.Path(), "watcher", j.watcher.Name)
				continue
			}
			if !watchedFilesSet.Contains(current.Path()) {
				j.logger.Debug("sync: adding new watched file", "path", current.Path())
				filename := filepath.Base(current.Path())
				addedFiles[filename] = current.Path()
			}
			onDisk.Add(current.Path())
		}
	}

	return onDisk, &addedFiles, nil
}
