-- Retry failed job runs and block repeated failures.

PRAGMA foreign_keys = OFF;

CREATE TABLE IF NOT EXISTS jobs_new (
    id TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    definition_of_done TEXT NOT NULL DEFAULT '',
    progress TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'ongoing', 'done', 'blocked')),
    workspace_path TEXT,
    assigned_session_id TEXT,
    lease_expires_at INTEGER,
    created_by TEXT NOT NULL DEFAULT 'user' CHECK (created_by IN ('user', 'agent')),
    source_session_id TEXT,
    source_platform TEXT NOT NULL DEFAULT '' CHECK (source_platform IN ('', 'desktop', 'discord')),
    source_channel_id TEXT,
    archived_at INTEGER,
    failure_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at INTEGER,
    last_failure_reason TEXT,
    deleted_at INTEGER,
    created_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000),
    updated_at INTEGER NOT NULL DEFAULT (unixepoch('now', 'subsec') * 1000)
);

INSERT INTO jobs_new (
    id, description, definition_of_done, progress, status, workspace_path,
    assigned_session_id, lease_expires_at, created_by, source_session_id,
    source_platform, source_channel_id, archived_at, failure_count,
    next_retry_at, last_failure_reason, deleted_at, created_at, updated_at
)
SELECT
    id, description, definition_of_done, progress, status, workspace_path,
    assigned_session_id, lease_expires_at, created_by, source_session_id,
    source_platform, source_channel_id, archived_at, 0,
    NULL, NULL, deleted_at, created_at, updated_at
FROM jobs;

DROP TABLE jobs;

ALTER TABLE jobs_new RENAME TO jobs;

CREATE INDEX IF NOT EXISTS idx_jobs_status_updated ON jobs (status, updated_at ASC);

CREATE INDEX IF NOT EXISTS idx_jobs_assigned_session ON jobs (assigned_session_id);

CREATE INDEX IF NOT EXISTS idx_jobs_deleted_at ON jobs (deleted_at);

CREATE INDEX IF NOT EXISTS idx_jobs_archived_at ON jobs (archived_at);

CREATE INDEX IF NOT EXISTS idx_jobs_next_retry_at ON jobs (next_retry_at);

PRAGMA foreign_keys = ON;
