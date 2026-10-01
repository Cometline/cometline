-- Archive completed jobs separately from deletion.

ALTER TABLE jobs ADD COLUMN archived_at INTEGER;

CREATE INDEX IF NOT EXISTS idx_jobs_archived_at ON jobs (archived_at);
