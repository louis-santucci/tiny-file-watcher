CREATE TABLE IF NOT EXISTS machines
(
    id              INTEGER PRIMARY KEY,
    token           TEXT UNIQUE NOT NULL,
    name            TEXT UNIQUE NOT NULL,
    ip              TEXT        NOT NULL,
    ssh_port        INTEGER     NOT NULL,
    ssh_user        TEXT        NOT NULL,
    ssh_private_key TEXT        NOT NULL,
    created_at      TEXT        NOT NULL,
    updated_at      TEXT        NOT NULL
);

CREATE TABLE IF NOT EXISTS file_watchers
(
    id           INTEGER PRIMARY KEY,
    machine_name TEXT        NOT NULL REFERENCES machines (name) ON DELETE CASCADE,
    name         TEXT UNIQUE NOT NULL,
    source_path  TEXT        NOT NULL,
    created_at   TEXT        NOT NULL,
    updated_at   TEXT        NOT NULL
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

-- View: all unflushed files paired with their watcher's redirection target.
-- Files whose watcher has no redirection are excluded and remain unflushed.
CREATE VIEW IF NOT EXISTS pending_file_flushes AS
SELECT wf.id           AS watched_file_id,
       m.name          AS machine_name,
       wf.watcher_name AS watcher_name,
       wf.file_path    AS file_path,
       wf.file_name    AS file_name,
       fr.target_path  AS target_path
FROM watched_files wf
         INNER JOIN file_redirections fr ON fr.watcher_name = wf.watcher_name
         INNER JOIN file_watchers fw ON fw.name = wf.watcher_name
         INNER JOIN machines m ON m.name = fw.machine_name
WHERE wf.flushed = 0;