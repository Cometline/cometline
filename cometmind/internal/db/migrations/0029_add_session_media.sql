-- Persisted session media catalog for gallery, video, and tombstones.

CREATE TABLE IF NOT EXISTS session_media (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('image', 'video')),
    media_type TEXT NOT NULL,
    alt TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    provider_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'generated' CHECK (
        source IN ('generated', 'presented', 'captured', 'imported', 'user')
    ),
    source_media_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready', 'deleted')),
    byte_size INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_session_media_gallery ON session_media (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_media_workspace ON session_media (workspace_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_media_session ON session_media (session_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_session_media_kind ON session_media (kind, status, created_at DESC);
