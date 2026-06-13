CREATE TABLE workspaces (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    path        TEXT NOT NULL UNIQUE,
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces (id),
    title        TEXT NOT NULL DEFAULT '',
    model_id     TEXT NOT NULL,
    provider_id  TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'archived')),
    token_usage  TEXT NOT NULL DEFAULT '{}',
    created_at   INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at   INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
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
