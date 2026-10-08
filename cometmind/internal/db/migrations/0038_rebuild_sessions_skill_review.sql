-- Allow every subagent_kind the runtime writes, including coding and skill_review,
-- and persist skill-review cooldown plus the previous turn's mutating targets.

PRAGMA foreign_keys = OFF;

CREATE TABLE sessions_new (
    id                 TEXT PRIMARY KEY,
    workspace_id       TEXT NOT NULL REFERENCES workspaces (id),
    title              TEXT NOT NULL DEFAULT '',
    model_id           TEXT NOT NULL,
    provider_id        TEXT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active', 'archived')),
    origin             TEXT NOT NULL DEFAULT 'user'
                       CHECK (origin IN ('user', 'autonomy', 'inbox')),
    is_disposable      INTEGER NOT NULL DEFAULT 1,
    token_usage        TEXT NOT NULL DEFAULT '{}',
    parent_session_id  TEXT REFERENCES sessions_new (id) ON DELETE SET NULL,
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
    subagent_kind      TEXT NOT NULL DEFAULT ''
                       CHECK (subagent_kind IN ('', 'general', 'acp', 'coding', 'skill_review')),
    agent_mode         TEXT NOT NULL DEFAULT 'auto'
                       CHECK (agent_mode IN ('auto', 'plan')),
    pinned             INTEGER NOT NULL DEFAULT 0,
    context_summary    TEXT NOT NULL DEFAULT '',
    compacted_until_message_id TEXT,
    context_summary_updated_at TEXT,
    skill_review_started_at INTEGER NOT NULL DEFAULT 0,
    skill_review_last_targets TEXT NOT NULL DEFAULT '',
    created_at         INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    updated_at         INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000)
);

INSERT INTO sessions_new (
    id, workspace_id, title, model_id, provider_id, status, origin, is_disposable,
    token_usage, parent_session_id, purpose, delegation_status, output_summary,
    subagent_kind, agent_mode, pinned, context_summary, compacted_until_message_id,
    context_summary_updated_at, skill_review_started_at, skill_review_last_targets,
    created_at, updated_at
)
SELECT
    id, workspace_id, title, model_id, provider_id, status, origin, is_disposable,
    token_usage, parent_session_id, purpose, delegation_status, output_summary,
    subagent_kind, agent_mode, pinned, context_summary, compacted_until_message_id,
    context_summary_updated_at, 0, '',
    created_at, updated_at
FROM sessions;

DROP TABLE sessions;

ALTER TABLE sessions_new RENAME TO sessions;

CREATE INDEX idx_sessions_workspace ON sessions (workspace_id);

CREATE INDEX idx_sessions_updated ON sessions (updated_at DESC);

CREATE INDEX idx_sessions_origin ON sessions (origin);

CREATE INDEX idx_sessions_parent ON sessions (parent_session_id);

PRAGMA foreign_keys = ON;
