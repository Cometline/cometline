CREATE TABLE workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    path        TEXT NOT NULL UNIQUE,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE sessions (
    id                 TEXT PRIMARY KEY,
    workspace_id       TEXT NOT NULL REFERENCES workspaces (id),
    title              TEXT NOT NULL DEFAULT '',
    model_id           TEXT NOT NULL,
    provider_id        TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'archived')),
    token_usage        TEXT NOT NULL DEFAULT '{}',
    parent_session_id  TEXT REFERENCES sessions (id) ON DELETE SET NULL,
    purpose            TEXT NOT NULL DEFAULT '',
    delegation_status  TEXT NOT NULL DEFAULT ''
                       CHECK (
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
    output_summary     TEXT NOT NULL DEFAULT '',
    acp_session_id     TEXT NOT NULL DEFAULT '',
    pending_question   TEXT NOT NULL DEFAULT '',
    created_at         INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at         INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE messages (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    role        TEXT NOT NULL
                CHECK (
                    role IN ('user', 'assistant', 'tool_result', 'system')
                ),
    content             TEXT NOT NULL DEFAULT '',
    reasoning_content   TEXT NOT NULL DEFAULT '[]',
    token_count INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE tool_calls (
    id          TEXT PRIMARY KEY,
    message_id  TEXT NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    tool_name   TEXT NOT NULL,
    arguments   TEXT NOT NULL DEFAULT '{}',
    result      TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    exit_code   INTEGER,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX idx_sessions_workspace ON sessions (workspace_id);

CREATE INDEX idx_sessions_updated ON sessions (updated_at DESC);

CREATE INDEX idx_messages_session ON messages (session_id, created_at);

CREATE INDEX idx_tool_calls_message ON tool_calls (message_id);

CREATE INDEX idx_sessions_parent ON sessions (parent_session_id);

CREATE TABLE gateway_sessions (
    id                   TEXT PRIMARY KEY,
    platform             TEXT NOT NULL,
    platform_user_id     TEXT NOT NULL,
    platform_channel_id  TEXT NOT NULL,
    thread_id            TEXT NOT NULL DEFAULT '',
    cometmind_session_id TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    workspace_id         TEXT NOT NULL REFERENCES workspaces (id),
    last_active_at       INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    UNIQUE (
        platform,
        platform_user_id,
        platform_channel_id,
        thread_id
    )
);

CREATE INDEX idx_gateway_sessions_lookup ON gateway_sessions (
    platform,
    platform_user_id,
    platform_channel_id,
    thread_id
);

CREATE TABLE memories (
    id                  TEXT PRIMARY KEY,
    scope               TEXT NOT NULL DEFAULT 'global',
    kind                TEXT NOT NULL DEFAULT 'fact',
    content             TEXT NOT NULL,
    embedding           BLOB,
    embedding_model     TEXT,
    source              TEXT NOT NULL,
    base_weight         REAL NOT NULL DEFAULT 1.0,
    access_count        INTEGER NOT NULL DEFAULT 0,
    pinned              INTEGER NOT NULL DEFAULT 0,
    source_session_id   TEXT,
    superseded_by       TEXT,
    archived            INTEGER NOT NULL DEFAULT 0,
    archived_reason     TEXT,
    last_accessed_at    INTEGER,
    created_at          INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at          INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX idx_memories_active ON memories (archived, scope);

CREATE INDEX idx_memories_weight ON memories (archived, base_weight);

CREATE TABLE memory_events (
    id          TEXT PRIMARY KEY,
    memory_id   TEXT,
    action      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);
