-- name: AcquireSessionRun :execrows
INSERT INTO session_runs (session_id, run_id, owner)
VALUES (?, ?, ?)
ON CONFLICT (session_id) DO NOTHING;

-- name: GetSessionRun :one
SELECT *
FROM session_runs
WHERE session_id = ?;

-- name: SessionRunExists :one
SELECT EXISTS (
    SELECT 1
    FROM session_runs
    WHERE session_id = ?
);

-- name: HeartbeatSessionRun :execrows
UPDATE session_runs
SET updated_at = unixepoch ('now', 'subsec') * 1000
WHERE session_id = ? AND run_id = ?;

-- name: RequestSessionRunAbort :execrows
UPDATE session_runs
SET
    abort_requested = 1,
    updated_at = unixepoch ('now', 'subsec') * 1000
WHERE session_id = ?;

-- name: ReleaseSessionRun :execrows
DELETE FROM session_runs
WHERE session_id = ? AND run_id = ?;

-- name: DeleteStaleSessionRun :execrows
DELETE FROM session_runs
WHERE session_id = ? AND updated_at < ?;
