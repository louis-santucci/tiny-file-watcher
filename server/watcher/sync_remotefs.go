package watcher

import (
	"io"
	"os"

	"github.com/kr/fs"
	"github.com/pkg/sftp"
)

// RemoteFS abstracts the file-system operations performed against the remote
// machine during a sync. In production the SFTP-backed implementation is
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
