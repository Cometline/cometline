-- name: CreateSession :one
INSERT INTO sessions (id, workspace_id, title, model_id, provider_id, status)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: CreateChildSession :one
INSERT INTO sessions (
    id,
    workspace_id,
    title,
    model_id,
    provider_id,
    status,
    parent_session_id,
    purpose,
    delegation_status,
    output_summary
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListChildSessions :many
SELECT *
FROM sessions
WHERE parent_session_id = ?
ORDER BY created_at ASC;

-- name: UpdateSessionDelegation :exec
UPDATE sessions
SET
    delegation_status = ?,
    output_summary = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: UpdateSessionDelegationState :exec
UPDATE sessions
SET
    delegation_status = ?,
    output_summary = ?,
    pending_question = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: UpdateSessionACP :exec
UPDATE sessions
SET
    acp_session_id = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: GetActiveChildForParent :one
SELECT *
FROM sessions
WHERE
    parent_session_id = ?
    AND delegation_status IN (
        'running',
        'awaiting_user',
        'awaiting_permission'
    )
ORDER BY updated_at DESC
LIMIT 1;

-- name: GetSession :one
SELECT *
FROM sessions
WHERE id = ?
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = ?;

-- name: ListSessionsByWorkspaceAsc :many
SELECT id, updated_at, delegation_status
FROM sessions
WHERE workspace_id = ?
ORDER BY updated_at ASC;

-- name: ListStaleSessionIDs :many
SELECT id
FROM sessions
WHERE
    workspace_id = ?
    AND updated_at < ?
    AND delegation_status NOT IN (
        'pending',
        'running',
        'awaiting_user',
        'awaiting_permission'
    )
ORDER BY updated_at ASC;

-- name: ListSessionsByWorkspace :many
SELECT *
FROM sessions
WHERE workspace_id = ?
ORDER BY updated_at DESC;

-- name: ListAllSessions :many
SELECT *
FROM sessions
WHERE parent_session_id IS NULL
ORDER BY updated_at DESC;

-- name: UpdateSessionTitle :exec
UPDATE sessions
SET
    title = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: SetTitleIfEmpty :exec
-- Atomically sets title only when the current title is blank.
-- The WHERE condition collapses the read-check-write into a single
-- SQLite statement, eliminating the TOCTOU race in SetTitleIfEmpty.
UPDATE sessions
SET
    title = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?
  AND trim(title) = '';

-- name: UpdateSessionTokenUsage :exec
UPDATE sessions
SET
    token_usage = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: UpdateSessionModel :exec
UPDATE sessions
SET
    model_id = ?,
    provider_id = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: UpdateSessionWorkspace :exec
UPDATE sessions
SET
    workspace_id = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: TouchSession :exec
UPDATE sessions
SET updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;
