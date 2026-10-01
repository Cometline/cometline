-- Subagent session fields and the gateway_sessions table.

ALTER TABLE sessions ADD COLUMN parent_session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL;

ALTER TABLE sessions ADD COLUMN purpose TEXT NOT NULL DEFAULT '';

ALTER TABLE sessions ADD COLUMN delegation_status TEXT NOT NULL DEFAULT '';

ALTER TABLE sessions ADD COLUMN output_summary TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions (parent_session_id);

CREATE TABLE IF NOT EXISTS gateway_sessions (
    id TEXT PRIMARY KEY,
    platform TEXT NOT NULL,
    platform_user_id TEXT NOT NULL,
    platform_channel_id TEXT NOT NULL,
    thread_id TEXT NOT NULL DEFAULT '',
    cometmind_session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id),
    last_active_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
    UNIQUE (platform, platform_user_id, platform_channel_id, thread_id)
);

CREATE INDEX IF NOT EXISTS idx_gateway_sessions_lookup ON gateway_sessions (
    platform, platform_user_id, platform_channel_id, thread_id
);
