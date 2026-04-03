package watcher_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tiny-file-watcher/server/watcher"
)

func setupIgnoreTest(t *testing.T, content string) (root string, ignorer watcher.Ignorer) {
	t.Helper()
	root = t.TempDir()
	if content != "" {
		err := os.WriteFile(filepath.Join(root, ".tfwignore"), []byte(content), 0644)
		require.NoError(t, err)
	}
	ignorer, err := watcher.LoadIgnore(root, slog.Default())
	require.NoError(t, err)
	return
}

func touch(t *testing.T, root, rel string) string {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0755))
	if !strings.HasSuffix(rel, "/") {
		require.NoError(t, os.WriteFile(full, nil, 0644))
	} else {
		require.NoError(t, os.MkdirAll(full, 0755))
	}
	return full
}

func TestLoadIgnore_NoFile_AcceptsAll(t *testing.T) {
	root := t.TempDir()
	ignorer, err := watcher.LoadIgnore(root, slog.Default())
	require.NoError(t, err)

	f := touch(t, root, "src/main.go")
	assert.False(t, ignorer.MatchesPath(root, f))
	assert.False(t, ignorer.MatchesPath(root, filepath.Join(root, "a/b/c.log")))
}

func TestLoadIgnore_EmptyFile_AcceptsAll(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "")
	f := touch(t, root, "src/main.go")
	assert.False(t, ignorer.MatchesPath(root, f))
}

func TestLoadIgnore_CommentsAndBlankLines_Ignored(t *testing.T) {
	content := `# This is a comment

# Another comment

`
	root, ignorer := setupIgnoreTest(t, content)
	f := touch(t, root, "# This is a comment")
	assert.False(t, ignorer.MatchesPath(root, f))
}

func TestIgnore_SimpleFilename(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "error.log\n")

	logFile := touch(t, root, "error.log")
	assert.True(t, ignorer.MatchesPath(root, logFile))

	jsonFile := touch(t, root, "data.json")
	assert.False(t, ignorer.MatchesPath(root, jsonFile))
}

func TestIgnore_ExtensionPattern(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "*.tmp\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "build.tmp")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "sub/build.tmp")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "build.go")))
}

func TestIgnore_DirectoryPattern(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "dist/\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "dist/bundle.js")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "dist/sub/bundle.js")))
	// A file called src/dist is not matched by dist/
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/dist")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/dist.js")))
}

func TestIgnore_NegationPattern(t *testing.T) {
	content := `*.log
!important.log
`
	root, ignorer := setupIgnoreTest(t, content)

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "debug.log")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "important.log")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "sub/other.log")))
}

func TestIgnore_DeepPathPattern(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "**/cache\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "cache/file.txt")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "a/b/cache/file.txt")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/cached.go")))
}

func TestIgnore_RootAnchoredPattern(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "/secret.txt\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "secret.txt")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "sub/secret.txt")))
}

func TestIgnore_MultiplePatterns(t *testing.T) {
	content := `.DS_Store
*.bak
Thumbs.db
`
	root, ignorer := setupIgnoreTest(t, content)

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, ".DS_Store")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "backup.bak")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "sub/Thumbs.db")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "readme.md")))
}

func TestIgnore_WildcardInDir(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "src/**/*.test.js\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "src/foo.test.js")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "src/a/b/bar.test.js")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/foo.js")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "test/foo.test.js")))
}

func TestIgnore_CaseSensitivity(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "Makefile\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "Makefile")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "makefile")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "MAKEFILE")))
}

func TestIgnore_DoubleStar(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "**/node_modules/**\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "node_modules/pkg/index.js")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "a/b/node_modules/pkg/index.js")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/index.js")))
}

func TestIgnore_NoMatchAccepts(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "*.log\n")

	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "readme.md")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/main.go")))
}

func TestIgnore_TrailingSpaces(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "foo.txt   \n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "foo.txt")))
}

func TestIgnore_EscapedSpace(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "foo\\ bar.txt\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "foo bar.txt")))
}

func TestIgnore_SlashInPattern_MatchesDirectoryRecursively(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "build/output/\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "build/output/app.js")))
	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "build/output/sub/app.js")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "build/input/app.js")))
}

func TestIgnore_PatternWithLeadingSlash_MatchesRootOnly(t *testing.T) {
	root, ignorer := setupIgnoreTest(t, "/log\n")

	assert.True(t, ignorer.MatchesPath(root, touch(t, root, "log")))
	assert.False(t, ignorer.MatchesPath(root, touch(t, root, "src/log")))
}
