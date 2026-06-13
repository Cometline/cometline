-- name: CreateToolCall :one
INSERT INTO tool_calls (
    id,
    message_id,
    tool_name,
    arguments,
    result,
    duration_ms,
    exit_code
)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateToolCallResult :exec
UPDATE tool_calls
SET
    result = ?,
    duration_ms = ?,
    exit_code = ?
WHERE id = ?;

-- name: ListToolCallsByMessage :many
SELECT *
FROM tool_calls
WHERE message_id = ?
ORDER BY created_at ASC;
