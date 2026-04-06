package watcher

import (
	"context"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	. "tiny-file-watcher/internal"
	"tiny-file-watcher/server/config"
	"tiny-file-watcher/server/database"

	"github.com/kr/fs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type SyncJob struct {
	watcher           *database.FileWatcher
	machine           *database.Machine
	logger            *slog.Logger
	sshConfig         *config.SSHConfig
	publicKey         ssh.PublicKey
	fileRepository    database.FileRepository
	watcherRepository database.FileWatcherRepository
	transactor        database.Transactor
	ignorer           Ignorer
}

type SyncResult struct {
	AddedCount   int32
	RemovedCount int32
	AddedFiles   []string
	RemovedFiles []string
}

func NewSyncJob(logger *slog.Logger, watcher *database.FileWatcher, machine *database.Machine, sshConfig *config.SSHConfig, publicKey ssh.PublicKey, fileRepo database.FileRepository, watcherRepo database.FileWatcherRepository, transactor database.Transactor, ignorer Ignorer) *SyncJob {
	return &SyncJob{
		watcher:           watcher,
		machine:           machine,
		logger:            logger,
		sshConfig:         sshConfig,
		publicKey:         publicKey,
		fileRepository:    fileRepo,
		watcherRepository: watcherRepo,
		transactor:        transactor,
		ignorer:           ignorer,
	}
}

func (j *SyncJob) Run() (*SyncResult, error) {
	j.logger.Info("starting sync job")

	var sshConfig = ssh.ClientConfig{
		User: j.machine.SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
				// read private key from disk and return it as a signer
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For simplicity; consider a more secure approach for production.
	}

	// SSH into the machines and sync the files for the given watcher
	sshUrl := j.machine.IP + ":" + string(j.machine.SSHPort)
	j.logger.Debug("sync: SSH URL: " + sshUrl)
	conn, err := ssh.Dial("tcp", sshUrl, &sshConfig)
	if err != nil {
		j.logger.Error("failed to connect to machine", "error", err)
		return nil, err
	}
	defer conn.Close()
	client, err := sftp.NewClient(conn)
	if err != nil {
		j.logger.Error("failed to create SFTP client", "error", err)
		return nil, err
	}
	watchedFiles, err := j.fileRepository.ListWatchedFiles(j.watcher.Name)
	watchedFilesSet := NewSetWithSize[string](len(watchedFiles))
	for _, watchedFile := range watchedFiles {
		watchedFilesSet.Add(watchedFile.FilePath)
	}

	if err != nil {
		j.logger.Error("failed to list watched files", "error", err)
		return nil, err
	}
	// walk the source path and check if the file exists in the db for this watcher, if not, create it, if yes, do nothing
	w := client.Walk(j.watcher.SourcePath)
	// using batch of results, check in db if file exists for this file watcher, if not, create it, if yes, do nothing

	onDisk, addedFiles, err := j.handleCurrentPaths(w, watchedFilesSet)
	if err != nil {
		j.logger.Error("sync: error handling current paths", "error", err, "watcher", j.watcher.Name)
		return nil, err
	}

	removedFiles := make([]string, 0)
	for _, watchedFile := range watchedFiles {
		if !onDisk.Contains(watchedFile.FilePath) {
			j.logger.Debug("sync: removing watched file that no longer exists on disk", "path", watchedFile.FilePath, "watcher", j.watcher.Name)
			removedFiles = append(removedFiles, watchedFile.FilePath)
		}
	}

	// bulk insert new files
	// bulk remove deleted files

	err = j.transactor.WithTransaction(context.Background(), func(repo database.TransactionalFileRepository) error {
		if len(*addedFiles) > 0 {
			_, err := repo.BulkAddWatchedFiles(j.watcher.Name, *addedFiles, false)
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

func (j *SyncJob) handleCurrentPaths(w *fs.Walker, watchedFilesSet *Set[string]) (*Set[string], *map[string]string, error) {
	onDisk := NewSet[string]()
	addedFiles := make(map[string]string)
	for w.Step() {
		if w.Err() != nil {
			j.logger.Error("sync: error walking source path", "error", w.Err(), "watcher", j.watcher.Name, "path", w.Path())
			continue
		}
		if w.Stat().IsDir() {
			j.logger.Debug("sync: skipping directory", "path", w.Path(), "watcher", j.watcher.Name)
			continue
		}
		if j.ignorer.MatchesPath(j.watcher.SourcePath, w.Path()) {
			j.logger.Debug("sync: skipping ignored file", "path", w.Path(), "watcher", j.watcher.Name)
			continue
		}
		if !watchedFilesSet.Contains(w.Path()) {
			j.logger.Debug("sync: adding new watched file", "path", w.Path())
			filename := filepath.Base(w.Path())
			addedFiles[filename] = w.Path()
		}
		onDisk.Add(w.Path())
	}

	return onDisk, &addedFiles, nil
}
