package flush

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"tiny-file-watcher/server/database"
	tfwssh "tiny-file-watcher/server/ssh"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FlushJobOption func(job *FlushJob)

// WithLogCallback sets a callback that is invoked with a human-readable
// progress message at each major step of the sync process.  Used by
// FlushWatcher to forward LOG events to the streaming client.
func WithLogCallback(fn func(string)) FlushJobOption {
	return func(j *FlushJob) {
		j.onLog = fn
	}
}

type FlushJob struct {
	watcherName     string
	logger          *slog.Logger
	flushRepository database.FlushRepository
	dialer          *SFTPDialer

	// onLog is an optional callback invoked with a human-readable message for each step of the job
	onLog func(string)
}

// clientPool caches open SFTP connections keyed by machine name.
type clientPool struct {
	dialer  *SFTPDialer
	clients map[string]SFTPClient
}

func (j *FlushJob) log(msg string) {
	if j.onLog != nil {
		j.onLog(msg)
	}
}

func newClientPool(dialer *SFTPDialer) *clientPool {
	return &clientPool{dialer: dialer, clients: make(map[string]SFTPClient)}
}

func NewFlushJob(logger *slog.Logger, repository database.FlushRepository, dialer *SFTPDialer, opts ...FlushJobOption) *FlushJob {
	job := &FlushJob{
		logger:          logger,
		flushRepository: repository,
		dialer:          dialer,
	}

	for _, opt := range opts {
		opt(job)
	}
	return job
}

func (j *FlushJob) Run() (bool, error) {
	j.logger.Info("starting flush job")

	pendingFiles, err := j.flushRepository.ListPendingFlushes(j.watcherName)
	if err != nil {
		return false, err
	}

	if len(pendingFiles) == 0 {
		j.log("no pending flushes")
		j.logger.Info("no pending flushes")
		return true, nil
	}

	pool := newClientPool(j.dialer)
	defer pool.closeAll()

	ids := make([]int64, 0, len(pendingFiles))
	for _, pf := range pendingFiles {
		if err := j.transferPendingFile(pool, pf); err != nil {
			return false, err
		}
		ids = append(ids, pf.WatchedFileID)
	}

	if err := j.flushRepository.FlushWatchedFiles(ids); err != nil {
		return false, status.Errorf(codes.Internal, "mark files flushed: %v", err)
	}

	return true, nil
}

func (j *FlushJob) transferPendingFile(pool *clientPool, pf *database.PendingFlush) error {
	srcClient, err := pool.get(pf.MachineName, pf.MachineIP, pf.MachineSSHPort, pf.MachineSSHUser, pf.MachineSSHPrivateKeyPath)
	if err != nil {
		return status.Errorf(codes.Internal, "connect to source machine %s: %v", pf.MachineName, err)
	}

	dstClient, err := pool.get(pf.TargetMachineName, pf.TargetMachineIP, pf.TargetMachineSSHPort, pf.TargetMachineSSHUser, pf.TargetMachineSSHPrivateKeyPath)
	if err != nil {
		return status.Errorf(codes.Internal, "connect to target machine %s: %v", pf.TargetMachineName, err)
	}

	src := filepath.Join(pf.FilePath, pf.FileName)
	dst := filepath.Join(pf.TargetPath, pf.FileName)
	if err := transferFile(srcClient, dstClient, src, dst); err != nil {
		return status.Errorf(codes.Internal, "transfer file %s: %v", src, err)
	}

	return nil
}

func (p *clientPool) get(name, ip string, port int32, user, keyPath string) (SFTPClient, error) {
	if c, ok := p.clients[name]; ok {
		return c, nil
	}
	c, err := (*p.dialer).Dial(tfwssh.MachineConfig{
		Name:              name,
		IP:                ip,
		SSHPort:           port,
		SSHUser:           user,
		SSHPrivateKeyPath: keyPath,
	})
	if err != nil {
		return nil, err
	}
	p.clients[name] = c
	return c, nil
}

func (p *clientPool) closeAll() {
	for _, c := range p.clients {
		c.Close()
	}
}

// transferFile copies a file from src on srcClient to dst on dstClient,
// preserving permissions and modification time.
func transferFile(srcClient, dstClient SFTPClient, src, dst string) error {
	srcInfo, err := srcClient.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if err := dstClient.MkdirAll(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("mkdir destination: %w", err)
	}

	in, err := srcClient.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	out, err := dstClient.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	if err := dstClient.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("chmod destination: %w", err)
	}

	modTime := srcInfo.ModTime()
	if err := dstClient.Chtimes(dst, modTime, modTime); err != nil {
		return fmt.Errorf("chtimes destination: %w", err)
	}

	return nil
}
