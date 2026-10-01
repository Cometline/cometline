-- Drop unused job scheduling/priority columns.

DROP INDEX IF EXISTS idx_jobs_status_priority;

DROP INDEX IF EXISTS idx_jobs_scheduled_at;

CREATE INDEX IF NOT EXISTS idx_jobs_status_updated ON jobs (status, updated_at ASC);

ALTER TABLE jobs DROP COLUMN priority;

ALTER TABLE jobs DROP COLUMN scheduled_at;

ALTER TABLE jobs DROP COLUMN due_at;
