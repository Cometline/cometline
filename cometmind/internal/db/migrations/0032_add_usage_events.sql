-- Usage ledger for the spend dashboard.

CREATE TABLE IF NOT EXISTS usage_events (
    id TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL DEFAULT (unixepoch ('now', 'subsec') * 1000),
    workspace_id TEXT REFERENCES workspaces (id) ON DELETE SET NULL,
    session_id TEXT REFERENCES sessions (id) ON DELETE SET NULL,
    provider_id TEXT NOT NULL,
    model_id TEXT NOT NULL,
    call_kind TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read INTEGER NOT NULL DEFAULT 0,
    cache_write INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_usage_events_created ON usage_events (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_events_model ON usage_events (provider_id, model_id);

CREATE INDEX IF NOT EXISTS idx_usage_events_workspace ON usage_events (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_events_kind ON usage_events (call_kind, created_at DESC);
