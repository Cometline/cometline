-- name: CreateSessionMedia :one
INSERT INTO session_media (
    id,
    session_id,
    storage_session_id,
    workspace_id,
    kind,
    media_type,
    alt,
    prompt,
    model,
    provider_id,
    source,
    source_media_id,
    status,
    byte_size,
    duration_ms
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSessionMedia :one
SELECT *
FROM session_media
WHERE id = ?
LIMIT 1;

-- name: GetReadySessionMedia :one
SELECT *
FROM session_media
WHERE id = ?
  AND session_id = ?
  AND status = 'ready'
LIMIT 1;

-- name: ListSessionMedia :many
SELECT *
FROM session_media
WHERE status = 'ready'
  AND (
    sqlc.narg ('workspace_id') IS NULL
    OR workspace_id = sqlc.narg ('workspace_id')
  )
  AND (
    sqlc.narg ('session_id') IS NULL
    OR session_id = sqlc.narg ('session_id')
  )
  AND (
    sqlc.narg ('kind') IS NULL
    OR kind = sqlc.narg ('kind')
  )
ORDER BY created_at DESC;

-- name: ListSessionMediaBySession :many
SELECT *
FROM session_media
WHERE session_id = ?
  AND status = 'ready'
ORDER BY created_at ASC;

-- name: MarkSessionMediaDeleted :one
UPDATE session_media
SET status = 'deleted'
WHERE id = ?
  AND status = 'ready'
RETURNING *;

-- name: UpdateSessionMediaWorkspace :exec
UPDATE session_media
SET workspace_id = ?
WHERE session_id = ?;

-- name: InitializeDetachedSessionMedia :exec
UPDATE session_media
SET detached_at = ?
WHERE session_id IS NULL
  AND status = 'ready'
  AND detached_at = 0;

-- name: ListExpiredDetachedSessionMediaIDs :many
SELECT id
FROM session_media
WHERE session_id IS NULL
  AND status = 'ready'
  AND detached_at > 0
  AND detached_at <= ?
ORDER BY detached_at ASC;
