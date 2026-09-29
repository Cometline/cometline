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

-- name: DeleteMessagesBySession :exec
DELETE FROM messages
WHERE session_id = ?;

-- name: UpdateMessageContent :exec
UPDATE messages
SET content = ?
WHERE id = ?;

-- name: ListTranscriptMessagesRecent :many
-- Visible transcript rows only (tool_result is joined separately for error flags).
SELECT *
FROM messages
WHERE session_id = sqlc.arg(session_id)
  AND role IN ('user', 'assistant', 'system')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListTranscriptMessagesBefore :many
-- Keyset: rows strictly older than (before_created_at, before_id).
SELECT *
FROM messages
WHERE session_id = sqlc.arg(session_id)
  AND role IN ('user', 'assistant', 'system')
  AND (
    created_at < sqlc.arg(before_created_at)
    OR (
      created_at = sqlc.arg(before_created_at)
      AND id < sqlc.arg(before_id)
    )
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListToolResultMessagesAfter :many
SELECT *
FROM messages
WHERE session_id = sqlc.arg(session_id)
  AND role = 'tool_result'
  AND created_at >= sqlc.arg(min_created_at)
ORDER BY created_at ASC, id ASC;

-- name: ListToolResultMessagesBetween :many
SELECT *
FROM messages
WHERE session_id = sqlc.arg(session_id)
  AND role = 'tool_result'
  AND created_at >= sqlc.arg(min_created_at)
  AND (
    created_at < sqlc.arg(max_created_at)
    OR (
      created_at = sqlc.arg(max_created_at)
      AND id < sqlc.arg(max_id)
    )
  )
ORDER BY created_at ASC, id ASC;
