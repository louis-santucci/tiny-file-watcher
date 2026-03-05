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
    detected_at TEXT    NOT NULL,
    flushed     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS file_redirections
(
    id          INTEGER PRIMARY KEY,
    watcher_id  INTEGER NOT NULL REFERENCES file_watchers (id) ON DELETE CASCADE,
    target_path TEXT    NOT NULL,
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_file_redirections_watcher_id ON file_redirections (watcher_id);

-- View: all unflushed files paired with their watcher's redirection target.
-- Files whose watcher has no redirection are excluded and remain unflushed.
CREATE VIEW IF NOT EXISTS pending_file_flushes AS
SELECT wf.id          AS watched_file_id,
       wf.watcher_id  AS watcher_id,
       fw.name        AS watcher_name,
       wf.file_path   AS file_path,
       wf.file_name   AS file_name,
       fr.id          AS redirection_id,
       fr.target_path AS target_path
FROM watched_files wf
         INNER JOIN file_redirections fr ON fr.watcher_id = wf.watcher_id
         INNER JOIN file_watchers fw ON fw.id = wf.watcher_id
WHERE wf.flushed = 0;