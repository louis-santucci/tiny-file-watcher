package watcher

import (
	"bufio"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	gitignore "github.com/sabhiram/go-gitignore"
)

const ignoreFileName = ".tfwignore"

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
func LoadIgnore(client sftp.Client, path string, logger *slog.Logger) (Ignorer, error) {
	ignoreFile, err := client.OpenFile(path, os.O_RDONLY)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Debug("no .tfwignore found, all files accepted", "path", path)
			return noopIgnorer{}, nil
		}
	}

	defer ignoreFile.Close()
	lines := []string{}
	fileScanner := bufio.NewScanner(ignoreFile)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		lines = append(lines, line)
	}

	compiled := gitignore.CompileIgnoreLines(lines...)
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
