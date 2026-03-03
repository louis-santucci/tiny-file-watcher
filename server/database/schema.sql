CREATE TABLE IF NOT EXISTS file_watchers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source_path TEXT NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS watched_files (
    id          TEXT PRIMARY KEY,
    watcher_id  TEXT NOT NULL REFERENCES file_watchers(id) ON DELETE CASCADE,
    file_name   TEXT NOT NULL,
    file_path   TEXT NOT NULL,
    detected_at TEXT NOT NULL
);