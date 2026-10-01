-- Track when Gallery media first becomes detached from a session.

ALTER TABLE session_media ADD COLUMN detached_at INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_session_media_detached ON session_media (session_id, status, detached_at);
