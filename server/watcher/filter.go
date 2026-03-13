package watcher

import (
	"path/filepath"
	"strings"

	"tiny-file-watcher/server/database"
)

// Evaluate decides whether a file at filePath should be accepted given the
// provided filters.
//
// Rules:
//  1. If no filters are defined, the file is accepted (backward-compatible).
//  2. If any exclude rule matches the file, the file is rejected.
//  3. If include rules exist, the file must match at least one to be accepted.
//  4. Exclude always takes precedence over include.
func Evaluate(filters []*database.WatcherFilter, filePath string) bool {
	if len(filters) == 0 {
		return true
	}

	fileName := filepath.Base(filePath)
	ext := strings.ToLower(filepath.Ext(fileName))

	hasIncludes := false
	matchedInclude := false

	for _, f := range filters {
		switch f.RuleType {
		case "exclude":
			if matchesPattern(f, fileName, ext) {
				return false
			}
		case "include":
			hasIncludes = true
			if matchesPattern(f, fileName, ext) {
				matchedInclude = true
			}
		}
	}

	if hasIncludes {
		return matchedInclude
	}
	return true
}

// matchesPattern returns true when the file (identified by its base name and
// lower-cased extension) satisfies the filter's pattern.
func matchesPattern(f *database.WatcherFilter, fileName, ext string) bool {
	switch f.PatternType {
	case "extension":
		// Normalise: accept ".mp3" or "mp3" in the pattern.
		p := strings.ToLower(f.Pattern)
		if !strings.HasPrefix(p, ".") {
			p = "." + p
		}
		return ext == p
	case "name":
		return strings.EqualFold(fileName, f.Pattern)
	case "glob":
		matched, err := filepath.Match(f.Pattern, fileName)
		return err == nil && matched
	}
	return false
}
