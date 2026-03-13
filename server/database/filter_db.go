package database

import "fmt"

// WatcherFilter mirrors the watcher_filters table row.
type WatcherFilter struct {
	ID          int64
	WatcherName string
	RuleType    string // "include" or "exclude"
	PatternType string // "extension", "name", or "glob"
	Pattern     string
}

// AddFilter inserts a new filter for a watcher.
func (db *DB) AddFilter(watcherName, ruleType, patternType, pattern string) (*WatcherFilter, error) {
	res, err := db.conn.Exec(
		`INSERT INTO watcher_filters (watcher_name, rule_type, pattern_type, pattern) VALUES (?,?,?,?)`,
		watcherName, ruleType, patternType, pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("add filter: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get inserted filter id: %w", err)
	}
	return &WatcherFilter{ID: id, WatcherName: watcherName, RuleType: ruleType, PatternType: patternType, Pattern: pattern}, nil
}

// GetFiltersForWatcher returns all filters for a given watcher name.
func (db *DB) GetFiltersForWatcher(watcherName string) ([]*WatcherFilter, error) {
	rows, err := db.conn.Query(
		`SELECT id, watcher_name, rule_type, pattern_type, pattern FROM watcher_filters WHERE watcher_name = ?`,
		watcherName,
	)
	if err != nil {
		return nil, fmt.Errorf("get filters: %w", err)
	}
	defer rows.Close()
	return scanFilters(rows)
}

// ListFilters returns all filters across all watchers.
func (db *DB) ListFilters() ([]*WatcherFilter, error) {
	rows, err := db.conn.Query(`SELECT id, watcher_name, rule_type, pattern_type, pattern FROM watcher_filters`)
	if err != nil {
		return nil, fmt.Errorf("list filters: %w", err)
	}
	defer rows.Close()
	return scanFilters(rows)
}

// DeleteFilter removes a filter by ID.
func (db *DB) DeleteFilter(id int64) error {
	res, err := db.conn.Exec(`DELETE FROM watcher_filters WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete filter: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("filter with id %d not found", id)
	}
	return nil
}

func scanFilters(rows interface {
	Scan(...any) error
	Next() bool
	Err() error
}) ([]*WatcherFilter, error) {
	var result []*WatcherFilter
	for rows.Next() {
		var f WatcherFilter
		if err := rows.Scan(&f.ID, &f.WatcherName, &f.RuleType, &f.PatternType, &f.Pattern); err != nil {
			return nil, fmt.Errorf("scan filter: %w", err)
		}
		result = append(result, &f)
	}
	return result, rows.Err()
}
