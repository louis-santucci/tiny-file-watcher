package watcher

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

const ignoreFileName = ".tfwignore"

// FileOpener is the file-open capability required by LoadIgnore.
// *sftp.Client satisfies this interface out of the box.
// In tests a local filesystem adapter can be used instead.
type FileOpener interface {
	OpenFile(path string, f int) (io.ReadCloser, error)
}

// Ignorer decides whether a file should be ignored during a sync.
// MatchesPath returns true when the file at the given absolute path must be
// excluded from the watcher.  The root parameter is the absolute path of the
// watcher's source directory.
type Ignorer interface {
	MatchesPath(root, absPath string) bool
}

// LoadIgnore loads the .tfwignore file at the given path and returns an Ignorer
// that can be used to check whether files should be ignored.  If the file does
// not exist, a noop Ignorer is returned that accepts all files.
func LoadIgnore(client FileOpener, path string, logger *slog.Logger) (Ignorer, error) {
	ignoreFile, err := client.OpenFile(path, os.O_RDONLY)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Warn("no .tfwignore found, all files accepted", "path", path)
			return noopIgnorer{}, nil
		}
		return nil, err
	}

	defer func() {
		if cerr := ignoreFile.Close(); cerr != nil {
			logger.Warn("load ignore: failed to close .tfwignore", "err", cerr)
		}
	}()
	lines := []string{}
	fileScanner := bufio.NewScanner(ignoreFile)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		lines = append(lines, line)
	}

	compiled := gitignore.CompileIgnoreLines(lines...)
	logger.Debug("load ignore: loaded .tfwignore", "path", path)
	return &fileIgnorer{compiled: compiled}, nil
}

// fileIgnorer wraps a compiled go-gitignore object.
type fileIgnorer struct {
	compiled *gitignore.GitIgnore
}

// MatchesPath returns true when absPath should be ignored according to the
// .tfwignore rules.  The path is evaluated relative to root, which matches
// the semantics of .gitignore (patterns are relative to the repo / source
// root).
func (fi *fileIgnorer) MatchesPath(root, absPath string) bool {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		// If we cannot compute a relative path, play it safe and accept.
		return false
	}
	// go-gitignore expects forward slashes on all platforms.
	rel = filepath.ToSlash(rel)
	// Strip any leading "./" that Rel may produce.
	rel = strings.TrimPrefix(rel, "./")
	return fi.compiled.MatchesPath(rel)
}

// noopIgnorer accepts every file — used when no .tfwignore is present.
type noopIgnorer struct{}

func (noopIgnorer) MatchesPath(_, _ string) bool { return false }
