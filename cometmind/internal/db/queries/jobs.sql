-- name: InsertJob :exec
INSERT INTO jobs (
    id,
    description,
    definition_of_done,
    progress,
    status,
    workspace_path,
    assigned_session_id,
    lease_expires_at,
    created_by,
    source_session_id,
    source_platform,
    source_channel_id,
    archived_at,
    failure_count,
    next_retry_at,
    last_failure_reason,
    deleted_at,
    scheduled_job_id,
    created_at,
    updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: HasOpenJobForScheduledJob :one
SELECT EXISTS (
    SELECT 1
    FROM jobs
    WHERE scheduled_job_id = ?
      AND status IN ('todo', 'ongoing', 'blocked')
      AND deleted_at IS NULL
) AS has_open_job;

-- name: GetJob :one
SELECT *
FROM jobs
WHERE id = ?;

-- name: ListJobs :many
SELECT *
FROM jobs
WHERE (sqlc.narg('include_deleted') = 1 OR deleted_at IS NULL)
  AND (sqlc.narg('include_archived') = 1 OR archived_at IS NULL)
  AND (sqlc.narg('status') IS NULL OR status = sqlc.narg('status'))
ORDER BY updated_at ASC;

-- name: ListReadyJobs :many
SELECT *
FROM jobs
WHERE deleted_at IS NULL
  AND archived_at IS NULL
  AND status = 'todo'
  AND (next_retry_at IS NULL OR next_retry_at <= ?)
ORDER BY updated_at ASC;

-- name: ListOngoingJobs :many
SELECT *
FROM jobs
WHERE deleted_at IS NULL
  AND archived_at IS NULL
  AND status = 'ongoing';

-- name: ListDeletedJobsBefore :many
SELECT id
FROM jobs
WHERE deleted_at IS NOT NULL
  AND deleted_at < ?;

-- name: ListArchivedJobsBefore :many
SELECT id
FROM jobs
WHERE deleted_at IS NULL
  AND archived_at IS NOT NULL
  AND archived_at < ?;

-- name: ListDoneJobsBefore :many
SELECT id
FROM jobs
WHERE deleted_at IS NULL
  AND archived_at IS NULL
  AND status = 'done'
  AND updated_at < ?;

-- name: UpdateJobTodoFields :execrows
UPDATE jobs
SET
    description = ?,
    definition_of_done = ?,
    workspace_path = ?,
    updated_at = ?
WHERE id = ?
  AND status = 'todo'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: UpdateJobProgress :execrows
UPDATE jobs
SET
    progress = ?,
    updated_at = ?
WHERE id = ?
  AND status = 'ongoing'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: ClaimJob :execrows
UPDATE jobs
SET
    status = 'ongoing',
    assigned_session_id = ?,
    lease_expires_at = ?,
    updated_at = ?
WHERE id = ?
  AND status = 'todo'
  AND deleted_at IS NULL
  AND archived_at IS NULL
  AND (next_retry_at IS NULL OR next_retry_at <= ?)
  AND assigned_session_id IS NULL;

-- name: ReleaseJob :execrows
UPDATE jobs
SET
    status = 'todo',
    assigned_session_id = NULL,
    lease_expires_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'ongoing'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: CompleteJob :execrows
UPDATE jobs
SET
    status = 'done',
    lease_expires_at = NULL,
    failure_count = 0,
    next_retry_at = NULL,
    last_failure_reason = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'ongoing'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: HeartbeatJob :execrows
UPDATE jobs
SET
    lease_expires_at = ?,
    updated_at = ?
WHERE id = ?
  AND status = 'ongoing'
  AND assigned_session_id = ?
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: ArchiveJob :execrows
UPDATE jobs
SET
    archived_at = ?,
    lease_expires_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'done'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: UnarchiveJob :execrows
UPDATE jobs
SET
    archived_at = NULL,
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL
  AND archived_at IS NOT NULL;

-- name: SoftDeleteJob :execrows
UPDATE jobs
SET
    deleted_at = ?,
    archived_at = NULL,
    failure_count = 0,
    next_retry_at = NULL,
    last_failure_reason = NULL,
    assigned_session_id = NULL,
    lease_expires_at = NULL,
    status = CASE WHEN status IN ('ongoing', 'blocked') THEN 'todo' ELSE status END,
    updated_at = ?
WHERE id = ?
  AND deleted_at IS NULL;

-- name: RecordJobFailure :execrows
UPDATE jobs
SET
    failure_count = failure_count + 1,
    next_retry_at = ?,
    last_failure_reason = ?,
    status = ?,
    assigned_session_id = NULL,
    lease_expires_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'ongoing'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: ResetJobFailures :execrows
UPDATE jobs
SET
    failure_count = 0,
    next_retry_at = NULL,
    last_failure_reason = NULL,
    status = 'todo',
    assigned_session_id = NULL,
    lease_expires_at = NULL,
    updated_at = ?
WHERE id = ?
  AND status = 'blocked'
  AND deleted_at IS NULL
  AND archived_at IS NULL;

-- name: HardDeleteJob :exec
DELETE FROM jobs
WHERE id = ?;

-- name: GetJobByAssignedSession :one
SELECT *
FROM jobs
WHERE assigned_session_id = ?
  AND status = 'ongoing'
  AND deleted_at IS NULL
  AND archived_at IS NULL
LIMIT 1;

-- name: InsertJobEvent :exec
INSERT INTO job_events (
    id,
    job_id,
    action,
    detail,
    actor_session_id,
    created_at
) VALUES (?, ?, ?, ?, ?, ?);

-- name: ListJobEvents :many
SELECT *
FROM job_events
WHERE job_id = ?
ORDER BY created_at ASC;
