-- name: InsertUsageEvent :exec
INSERT INTO usage_events (
    id,
    created_at,
    workspace_id,
    session_id,
    provider_id,
    model_id,
    call_kind,
    input_tokens,
    output_tokens,
    cache_read,
    cache_write
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListUsageEventsInRange :many
SELECT usage_events.*
FROM usage_events
WHERE usage_events.created_at >= sqlc.arg ('from_ms')
  AND usage_events.created_at < sqlc.arg ('to_ms')
  AND (
    sqlc.narg ('workspace_id') IS NULL
    OR usage_events.workspace_id = sqlc.narg ('workspace_id')
    OR usage_events.session_id IN (
      SELECT id FROM sessions WHERE workspace_id = sqlc.narg ('workspace_id')
    )
  )
ORDER BY usage_events.created_at ASC;

-- name: ListUsageEventsPage :many
SELECT usage_events.*
FROM usage_events
WHERE usage_events.created_at >= sqlc.arg ('from_ms')
  AND usage_events.created_at < sqlc.arg ('to_ms')
  AND (
    sqlc.narg ('workspace_id') IS NULL
    OR usage_events.workspace_id = sqlc.narg ('workspace_id')
    OR usage_events.session_id IN (
      SELECT id FROM sessions WHERE workspace_id = sqlc.narg ('workspace_id')
    )
  )
ORDER BY usage_events.created_at DESC
LIMIT sqlc.arg ('limit')
OFFSET sqlc.arg ('offset');

-- name: CountUsageEventsInRange :one
SELECT COUNT(*)
FROM usage_events
WHERE usage_events.created_at >= sqlc.arg ('from_ms')
  AND usage_events.created_at < sqlc.arg ('to_ms')
  AND (
    sqlc.narg ('workspace_id') IS NULL
    OR usage_events.workspace_id = sqlc.narg ('workspace_id')
    OR usage_events.session_id IN (
      SELECT id FROM sessions WHERE workspace_id = sqlc.narg ('workspace_id')
    )
  );

-- name: DeleteUsageEventsBefore :execrows
DELETE FROM usage_events
WHERE created_at < sqlc.arg ('before_ms');
