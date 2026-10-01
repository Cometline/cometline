-- Scheduled one-shot job definitions.

CREATE TABLE IF NOT EXISTS scheduled_jobs (
    id TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    definition_of_done TEXT NOT NULL DEFAULT '',
    workspace_path TEXT,
    created_by TEXT NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'agent')),
    source_session_id TEXT,
    source_platform TEXT NOT NULL DEFAULT '' CHECK (source_platform IN ('', 'desktop', 'discord')),
    source_channel_id TEXT,
    cron_expr TEXT,
    run_at INTEGER,
    next_run_at INTEGER NOT NULL,
    last_run_at INTEGER,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_due ON scheduled_jobs (enabled, next_run_at);

CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_updated ON scheduled_jobs (updated_at DESC);
