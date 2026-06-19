-- name: CreateMessage :one
INSERT INTO messages (id, session_id, role, content, reasoning_content, injected_memories, token_count)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListMessagesBySession :many
SELECT *
FROM messages
WHERE session_id = ?
ORDER BY created_at ASC, id ASC;

-- name: GetMessage :one
SELECT *
FROM messages
WHERE id = ?
LIMIT 1;
