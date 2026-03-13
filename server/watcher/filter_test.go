package watcher_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tiny-file-watcher/server/database"
	"tiny-file-watcher/server/watcher"
)

func filter(ruleType, patternType, pattern string) *database.WatcherFilter {
	return &database.WatcherFilter{RuleType: ruleType, PatternType: patternType, Pattern: pattern}
}

func TestEvaluate_NoFilters_AcceptsAll(t *testing.T) {
	assert.True(t, watcher.Evaluate(nil, "/music/song.mp3"))
	assert.True(t, watcher.Evaluate([]*database.WatcherFilter{}, "/any/file.xyz"))
}

func TestEvaluate_IncludeExtension_Accept(t *testing.T) {
	filters := []*database.WatcherFilter{filter("include", "extension", ".mp3")}
	assert.True(t, watcher.Evaluate(filters, "/music/track.mp3"))
}

func TestEvaluate_IncludeExtension_NormalisedWithoutDot(t *testing.T) {
	filters := []*database.WatcherFilter{filter("include", "extension", "mp3")}
	assert.True(t, watcher.Evaluate(filters, "/music/track.mp3"))
}

func TestEvaluate_IncludeExtension_Reject(t *testing.T) {
	filters := []*database.WatcherFilter{filter("include", "extension", ".mp3")}
	assert.False(t, watcher.Evaluate(filters, "/music/track.wav"))
}

func TestEvaluate_IncludeMultipleExtensions_Accept(t *testing.T) {
	filters := []*database.WatcherFilter{
		filter("include", "extension", ".mp3"),
		filter("include", "extension", ".wav"),
		filter("include", "extension", ".aiff"),
	}
	assert.True(t, watcher.Evaluate(filters, "/a/track.wav"))
	assert.True(t, watcher.Evaluate(filters, "/a/track.aiff"))
	assert.False(t, watcher.Evaluate(filters, "/a/track.txt"))
}

func TestEvaluate_ExcludeExtension_Reject(t *testing.T) {
	filters := []*database.WatcherFilter{filter("exclude", "extension", ".tmp")}
	assert.False(t, watcher.Evaluate(filters, "/tmp/file.tmp"))
}

func TestEvaluate_ExcludeExtension_Accept(t *testing.T) {
	filters := []*database.WatcherFilter{filter("exclude", "extension", ".tmp")}
	assert.True(t, watcher.Evaluate(filters, "/music/track.mp3"))
}

func TestEvaluate_ExcludeName_Reject(t *testing.T) {
	filters := []*database.WatcherFilter{filter("exclude", "name", ".DS_Store")}
	assert.False(t, watcher.Evaluate(filters, "/any/.DS_Store"))
}

func TestEvaluate_ExcludeGlob_Reject(t *testing.T) {
	filters := []*database.WatcherFilter{filter("exclude", "glob", "*.tmp")}
	assert.False(t, watcher.Evaluate(filters, "/dir/file.tmp"))
}

func TestEvaluate_ExcludeGlob_Accept(t *testing.T) {
	filters := []*database.WatcherFilter{filter("exclude", "glob", "*.tmp")}
	assert.True(t, watcher.Evaluate(filters, "/dir/track.mp3"))
}

func TestEvaluate_ExcludeTakesPrecedenceOverInclude(t *testing.T) {
	filters := []*database.WatcherFilter{
		filter("include", "extension", ".mp3"),
		filter("exclude", "extension", ".mp3"),
	}
	// exclude wins even though file matches an include rule
	assert.False(t, watcher.Evaluate(filters, "/music/track.mp3"))
}

func TestEvaluate_IncludeNameExact(t *testing.T) {
	filters := []*database.WatcherFilter{filter("include", "name", "cover.jpg")}
	assert.True(t, watcher.Evaluate(filters, "/album/cover.jpg"))
	assert.False(t, watcher.Evaluate(filters, "/album/other.jpg"))
}

func TestEvaluate_ExcludeOnly_AcceptsNonMatching(t *testing.T) {
	filters := []*database.WatcherFilter{filter("exclude", "extension", ".log")}
	// no include rules → everything passes except .log
	assert.True(t, watcher.Evaluate(filters, "/app/data.json"))
	assert.False(t, watcher.Evaluate(filters, "/app/error.log"))
}
