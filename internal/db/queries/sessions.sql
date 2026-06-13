-- name: CreateSession :one
INSERT INTO sessions (id, workspace_id, title, model_id, provider_id, status)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSession :one
SELECT *
FROM sessions
WHERE id = ?
LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = ?;

-- name: ListSessionsByWorkspace :many
SELECT *
FROM sessions
WHERE workspace_id = ?
ORDER BY updated_at DESC;

-- name: UpdateSessionTitle :exec
UPDATE sessions
SET
    title = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: UpdateSessionTokenUsage :exec
UPDATE sessions
SET
    token_usage = ?,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;

-- name: TouchSession :exec
UPDATE sessions
SET updated_at = unixepoch ('now', 'subsec') * 1000
WHERE id = ?;
