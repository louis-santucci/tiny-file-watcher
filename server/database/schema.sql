CREATE TABLE IF NOT EXISTS file_watchers
(
    id          INTEGER PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    source_path TEXT        NOT NULL,
    enabled     INTEGER     NOT NULL DEFAULT 0,
    created_at  TEXT        NOT NULL,
    updated_at  TEXT        NOT NULL
);

CREATE TABLE IF NOT EXISTS watched_files
(
    id          INTEGER PRIMARY KEY,
    watcher_id  INTEGER NOT NULL REFERENCES file_watchers (id) ON DELETE CASCADE,
    file_name   TEXT    NOT NULL,
    file_path   TEXT    NOT NULL,
    detected_at TEXT    NOT NULL
);