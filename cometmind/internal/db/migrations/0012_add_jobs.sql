-- Global jobs queue.

CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    definition_of_done TEXT NOT NULL DEFAULT '',
    progress TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'ongoing', 'done')),
    priority INTEGER NOT NULL DEFAULT 0,
    scheduled_at INTEGER,
    due_at INTEGER,
    workspace_path TEXT,
    assigned_session_id TEXT,
    lease_expires_at INTEGER,
    created_by TEXT NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'agent')),
    source_session_id TEXT,
    source_platform TEXT NOT NULL DEFAULT '' CHECK (source_platform IN ('', 'desktop', 'discord')),
    source_channel_id TEXT,
    deleted_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_jobs_status_priority ON jobs (status, priority DESC, updated_at ASC);

CREATE INDEX IF NOT EXISTS idx_jobs_assigned_session ON jobs (assigned_session_id);

CREATE INDEX IF NOT EXISTS idx_jobs_deleted_at ON jobs (deleted_at);

CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_at ON jobs (scheduled_at);

CREATE TABLE IF NOT EXISTS job_events (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    actor_session_id TEXT,
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_job_events_job ON job_events (job_id, created_at);
