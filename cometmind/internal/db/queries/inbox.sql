-- name: CreateInboxMessage :one
INSERT INTO inbox_messages (
    id,
    title,
    body,
    workspace_id,
    job_id,
    session_id,
    status,
    created_at,
    updated_at
)
VALUES (?, ?, ?, ?, ?, ?, 'open', ?, ?)
RETURNING *;

-- name: GetInboxMessage :one
SELECT *
FROM inbox_messages
WHERE id = ?
  AND deleted_at IS NULL
LIMIT 1;

-- name: ListInboxMessages :many
SELECT *
FROM inbox_messages
WHERE deleted_at IS NULL
  AND (
    sqlc.narg ('status') IS NULL
    OR status = sqlc.narg ('status')
  )
ORDER BY created_at DESC;

-- name: CountOpenInboxMessages :one
SELECT COUNT(*)
FROM inbox_messages
WHERE status = 'open'
  AND deleted_at IS NULL;

-- name: ReplyInboxMessage :one
UPDATE inbox_messages
SET
    status = 'archived',
    archive_reason = 'replied',
    user_reply = ?,
    archived_at = ?,
    updated_at = ?
WHERE id = ?
  AND status = 'open'
  AND deleted_at IS NULL
RETURNING *;

-- name: DismissInboxMessage :one
UPDATE inbox_messages
SET
    status = 'archived',
    archive_reason = 'dismissed',
    archived_at = ?,
    updated_at = ?
WHERE id = ?
  AND status = 'open'
  AND deleted_at IS NULL
RETURNING *;

-- name: ListInboxMessagesPendingProcess :many
SELECT *
FROM inbox_messages
WHERE status = 'archived'
  AND archive_reason = 'replied'
  AND processed_at IS NULL
  AND deleted_at IS NULL
  AND process_attempts < ?
ORDER BY archived_at ASC
LIMIT ?;

-- name: ClaimInboxMessageForProcess :one
UPDATE inbox_messages
SET
    process_attempts = process_attempts + 1,
    updated_at = ?
WHERE id = ?
  AND status = 'archived'
  AND archive_reason = 'replied'
  AND processed_at IS NULL
  AND deleted_at IS NULL
  AND process_attempts < ?
RETURNING *;

-- name: MarkInboxMessageProcessed :one
UPDATE inbox_messages
SET
    processed_at = ?,
    process_error = ?,
    updated_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteExpiredInboxMessages :execrows
DELETE FROM inbox_messages
WHERE archived_at IS NOT NULL
  AND archived_at < ?;
