-- Link materialized jobs back to the scheduled job that created them, so a
-- schedule with an outstanding (todo/ongoing) job isn't re-materialized into
-- a duplicate job on the next due tick.

ALTER TABLE jobs ADD COLUMN scheduled_job_id TEXT REFERENCES scheduled_jobs (id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_scheduled_job_open ON jobs (scheduled_job_id, status)
    WHERE scheduled_job_id IS NOT NULL;
