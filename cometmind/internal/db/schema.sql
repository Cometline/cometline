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
    origin             TEXT NOT NULL DEFAULT 'user'
                       CHECK (origin IN ('user', 'autonomy')),
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
    subagent_kind      TEXT NOT NULL DEFAULT ''
                       CHECK (subagent_kind IN ('', 'general', 'acp')),
    pinned             INTEGER NOT NULL DEFAULT 0,
    context_summary    TEXT NOT NULL DEFAULT '',
    compacted_until_message_id TEXT,
    context_summary_updated_at TEXT,
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
    injected_memories   TEXT NOT NULL DEFAULT '[]',
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

CREATE INDEX idx_sessions_origin ON sessions (origin);

CREATE TABLE session_plans (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    step_index      INTEGER NOT NULL,
    description     TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'in_progress', 'completed', 'blocked')),
    blocker_reason  TEXT NOT NULL DEFAULT '',
    dismissed_at    INTEGER,
    created_at      INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at      INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    UNIQUE (session_id, step_index)
);

CREATE INDEX idx_session_plans_session ON session_plans (session_id, step_index);

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
    preference_category TEXT NOT NULL DEFAULT '',
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

CREATE INDEX idx_memories_preference_category ON memories (
    archived,
    kind,
    preference_category,
    updated_at DESC
);

CREATE INDEX idx_memories_kind_created ON memories (archived, kind, created_at DESC);

CREATE TABLE memory_events (
    id          TEXT PRIMARY KEY,
    memory_id   TEXT,
    action      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE VIRTUAL TABLE memories_fts USING fts5 (
    memory_id UNINDEXED,
    content
);

CREATE TABLE jobs (
    id                  TEXT PRIMARY KEY,
    description         TEXT NOT NULL,
    definition_of_done  TEXT NOT NULL DEFAULT '',
    progress            TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'todo'
                        CHECK (status IN ('todo', 'ongoing', 'done', 'blocked')),
    workspace_path      TEXT,
    assigned_session_id TEXT,
    lease_expires_at    INTEGER,
    created_by          TEXT NOT NULL DEFAULT 'user'
                        CHECK (created_by IN ('user', 'agent')),
    source_session_id   TEXT,
    source_platform     TEXT NOT NULL DEFAULT ''
                        CHECK (source_platform IN ('', 'desktop', 'discord')),
    source_channel_id   TEXT,
    archived_at         INTEGER,
    failure_count       INTEGER NOT NULL DEFAULT 0,
    next_retry_at       INTEGER,
    last_failure_reason TEXT,
    deleted_at          INTEGER,
    scheduled_job_id    TEXT REFERENCES scheduled_jobs (id) ON DELETE SET NULL,
    created_at          INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at          INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX idx_jobs_status_updated ON jobs (status, updated_at ASC);

CREATE INDEX idx_jobs_assigned_session ON jobs (assigned_session_id);

CREATE INDEX idx_jobs_deleted_at ON jobs (deleted_at);

CREATE INDEX idx_jobs_archived_at ON jobs (archived_at);

CREATE INDEX idx_jobs_next_retry_at ON jobs (next_retry_at);

CREATE INDEX idx_jobs_scheduled_job_open ON jobs (scheduled_job_id, status)
    WHERE scheduled_job_id IS NOT NULL;

CREATE TABLE scheduled_jobs (
    id                 TEXT PRIMARY KEY,
    description        TEXT NOT NULL,
    definition_of_done TEXT NOT NULL DEFAULT '',
    workspace_path     TEXT,
    created_by         TEXT NOT NULL DEFAULT 'user'
                       CHECK (created_by IN ('user', 'agent')),
    source_session_id  TEXT,
    source_platform    TEXT NOT NULL DEFAULT ''
                       CHECK (source_platform IN ('', 'desktop', 'discord')),
    source_channel_id  TEXT,
    cron_expr          TEXT,
    run_at             INTEGER,
    next_run_at        INTEGER NOT NULL,
    last_run_at        INTEGER,
    enabled            INTEGER NOT NULL DEFAULT 1,
    created_at         INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at         INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX idx_scheduled_jobs_due ON scheduled_jobs (enabled, next_run_at);

CREATE INDEX idx_scheduled_jobs_updated ON scheduled_jobs (updated_at DESC);

CREATE TABLE job_events (
    id                TEXT PRIMARY KEY,
    job_id            TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    action            TEXT NOT NULL,
    detail            TEXT NOT NULL DEFAULT '',
    actor_session_id  TEXT,
    created_at        INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

CREATE INDEX idx_job_events_job ON job_events (job_id, created_at);
