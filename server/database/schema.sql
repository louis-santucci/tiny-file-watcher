CREATE TABLE IF NOT EXISTS file_watchers
(
    id          INTEGER PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    source_path TEXT        NOT NULL,
    created_at  TEXT        NOT NULL,
    updated_at  TEXT        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_file_watchers_name ON file_watchers (name);

CREATE TABLE IF NOT EXISTS watched_files
(
    id           INTEGER PRIMARY KEY,
    watcher_name TEXT    NOT NULL REFERENCES file_watchers (name) ON DELETE CASCADE,
    file_name    TEXT    NOT NULL,
    file_path    TEXT    NOT NULL,
    detected_at  TEXT    NOT NULL,
    flushed      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS file_redirections
(
    watcher_name TEXT PRIMARY KEY REFERENCES file_watchers (name) ON DELETE CASCADE,
    target_path  TEXT    NOT NULL,
    auto_flush   INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL,
    updated_at   TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS watcher_filters
(
    id           INTEGER PRIMARY KEY,
    watcher_name TEXT NOT NULL REFERENCES file_watchers (name) ON DELETE CASCADE,
    rule_type    TEXT NOT NULL CHECK (rule_type IN ('include', 'exclude')),
    pattern_type TEXT NOT NULL CHECK (pattern_type IN ('extension', 'name', 'glob')),
    pattern      TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_watcher_filters_name ON watcher_filters (watcher_name);

-- View: per-watcher filter summary with include and exclude filters as two aggregated columns.
CREATE VIEW IF NOT EXISTS watcher_filters_summary AS
SELECT fw.id                                                                    AS watcher_id,
       fw.name                                                                  AS watcher_name,
       fw.source_path                                                           AS source_path,
       group_concat(CASE WHEN wf.rule_type = 'include' THEN wf.pattern_type || ':' || wf.pattern END,
                    ', ')                                                       AS include_filters,
       group_concat(CASE WHEN wf.rule_type = 'exclude' THEN wf.pattern_type || ':' || wf.pattern END,
                    ', ')                                                       AS exclude_filters
FROM file_watchers fw
         LEFT JOIN watcher_filters wf ON wf.watcher_name = fw.name
GROUP BY fw.id, fw.name, fw.source_path;

-- View: all unflushed files paired with their watcher's redirection target.
-- Files whose watcher has no redirection are excluded and remain unflushed.
CREATE VIEW IF NOT EXISTS pending_file_flushes AS
SELECT wf.id           AS watched_file_id,
       wf.watcher_name AS watcher_name,
       wf.file_path    AS file_path,
       wf.file_name    AS file_name,
       fr.target_path  AS target_path
FROM watched_files wf
         INNER JOIN file_redirections fr ON fr.watcher_name = wf.watcher_name
WHERE wf.flushed = 0;