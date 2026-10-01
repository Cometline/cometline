-- Keep gallery media after the owning workspace is deleted.

PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS session_media_new;

CREATE TABLE session_media_new (
    id TEXT PRIMARY KEY,
    session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL,
    storage_session_id TEXT NOT NULL,
    workspace_id TEXT REFERENCES workspaces (id) ON DELETE SET NULL,
    kind TEXT NOT NULL CHECK (kind IN ('image', 'video')),
    media_type TEXT NOT NULL,
    alt TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    provider_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'generated' CHECK (
        source IN (
            'generated',
            'presented',
            'captured',
            'imported',
            'user'
        )
    ),
    source_media_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'deleted')),
    byte_size INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

INSERT INTO session_media_new (
    id, session_id, storage_session_id, workspace_id, kind, media_type, alt,
    prompt, model, provider_id, source, source_media_id, status, byte_size,
    duration_ms, created_at
)
SELECT
    id, session_id, storage_session_id, workspace_id, kind, media_type, alt,
    prompt, model, provider_id, source, source_media_id, status, byte_size,
    duration_ms, created_at
FROM session_media;

DROP TABLE session_media;

ALTER TABLE session_media_new RENAME TO session_media;

CREATE INDEX IF NOT EXISTS idx_session_media_gallery ON session_media (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_media_workspace ON session_media (workspace_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_media_session ON session_media (session_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_media_kind ON session_media (kind, status, created_at DESC);

PRAGMA foreign_keys = ON;
