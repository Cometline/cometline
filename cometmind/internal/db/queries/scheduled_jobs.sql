-- name: InsertScheduledJob :exec
INSERT INTO scheduled_jobs (
  id,
  description,
  definition_of_done,
  workspace_path,
  created_by,
  source_session_id,
  source_platform,
  source_channel_id,
  cron_expr,
  run_at,
  next_run_at,
  last_run_at,
  enabled,
  created_at,
  updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetScheduledJob :one
SELECT * FROM scheduled_jobs WHERE id = ?;

-- name: ListScheduledJobs :many
SELECT * FROM scheduled_jobs
ORDER BY enabled DESC, next_run_at ASC, updated_at DESC;

-- name: ListDueScheduledJobs :many
SELECT * FROM scheduled_jobs
WHERE enabled = 1 AND next_run_at <= ?
ORDER BY next_run_at ASC;

-- name: UpdateScheduledJob :execrows
UPDATE scheduled_jobs
SET description = ?,
    definition_of_done = ?,
    workspace_path = ?,
    cron_expr = ?,
    run_at = ?,
    next_run_at = ?,
    enabled = ?,
    updated_at = ?
WHERE id = ?;

-- name: MarkScheduledJobFired :execrows
UPDATE scheduled_jobs
SET last_run_at = ?,
    enabled = 0,
    updated_at = ?
WHERE id = ? AND enabled = 1;

-- name: AdvanceScheduledJob :execrows
UPDATE scheduled_jobs
SET last_run_at = ?,
    next_run_at = ?,
    updated_at = ?
WHERE id = ? AND enabled = 1;

-- name: DeleteScheduledJob :execrows
DELETE FROM scheduled_jobs WHERE id = ?;
