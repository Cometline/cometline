-- name: UpsertGatewaySession :one
INSERT INTO gateway_sessions (
    id,
    platform,
    platform_user_id,
    platform_channel_id,
    thread_id,
    cometmind_session_id,
    workspace_id,
    last_active_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, unixepoch ('now', 'subsec') * 1000)
ON CONFLICT (
    platform,
    platform_user_id,
    platform_channel_id,
    thread_id
) DO UPDATE SET
    cometmind_session_id = excluded.cometmind_session_id,
    workspace_id = excluded.workspace_id,
    last_active_at = unixepoch ('now', 'subsec') * 1000
RETURNING *;

-- name: GetGatewaySession :one
SELECT *
FROM gateway_sessions
WHERE
    platform = ?
    AND platform_user_id = ?
    AND platform_channel_id = ?
    AND thread_id = ?
LIMIT 1;

-- name: UpdateGatewaySessionWorkspace :exec
UPDATE gateway_sessions
SET
    workspace_id = ?,
    last_active_at = unixepoch ('now', 'subsec') * 1000
WHERE cometmind_session_id = ?;
