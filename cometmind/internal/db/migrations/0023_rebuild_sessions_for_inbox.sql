-- Agent inbox, and sessions.origin allows 'inbox'.

PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS sessions_new (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id),
    title TEXT NOT NULL DEFAULT '',
    model_id TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    origin TEXT NOT NULL DEFAULT 'user' CHECK (origin IN ('user', 'autonomy', 'inbox')),
    token_usage TEXT NOT NULL DEFAULT '{}',
    parent_session_id TEXT REFERENCES sessions_new (id) ON DELETE SET NULL,
    purpose TEXT NOT NULL DEFAULT '',
    delegation_status TEXT NOT NULL DEFAULT '' CHECK (
        delegation_status IN (
            '',
            'pending',
            'running',
            'awaiting_user',
            'awaiting_permission',
            'completed',
            'failed',
            'cancelled'
        )
    ),
    output_summary TEXT NOT NULL DEFAULT '',
    acp_session_id TEXT NOT NULL DEFAULT '',
    pending_question TEXT NOT NULL DEFAULT '',
    subagent_kind TEXT NOT NULL DEFAULT '' CHECK (subagent_kind IN ('', 'general', 'acp')),
    pinned INTEGER NOT NULL DEFAULT 0,
    context_summary TEXT NOT NULL DEFAULT '',
    compacted_until_message_id TEXT,
    context_summary_updated_at TEXT,
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);

INSERT INTO sessions_new (
    id, workspace_id, title, model_id, provider_id, status, origin, token_usage,
    parent_session_id, purpose, delegation_status, output_summary, acp_session_id,
    pending_question, subagent_kind, pinned, context_summary,
    compacted_until_message_id, context_summary_updated_at, created_at, updated_at
)
SELECT
    id, workspace_id, title, model_id, provider_id, status, origin, token_usage,
    parent_session_id, purpose, delegation_status, output_summary, acp_session_id,
    pending_question, subagent_kind, pinned, context_summary,
    compacted_until_message_id, context_summary_updated_at, created_at, updated_at
FROM sessions;

DROP TABLE sessions;

ALTER TABLE sessions_new RENAME TO sessions;

CREATE INDEX IF NOT EXISTS idx_sessions_workspace ON sessions (workspace_id);

CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_origin ON sessions (origin);

CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions (parent_session_id);

CREATE TABLE IF NOT EXISTS inbox_messages (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    workspace_id TEXT,
    job_id TEXT,
    session_id TEXT,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'archived')),
    archive_reason TEXT CHECK (
        archive_reason IS NULL OR archive_reason IN ('replied', 'dismissed')
    ),
    user_reply TEXT,
    processed_at INTEGER,
    process_error TEXT,
    process_attempts INTEGER NOT NULL DEFAULT 0,
    archived_at INTEGER,
    deleted_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_inbox_messages_status_created ON inbox_messages (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_inbox_messages_process ON inbox_messages (
    status, archive_reason, processed_at, process_attempts
);

CREATE INDEX IF NOT EXISTS idx_inbox_messages_archived_at ON inbox_messages (archived_at);

PRAGMA foreign_keys = ON;
