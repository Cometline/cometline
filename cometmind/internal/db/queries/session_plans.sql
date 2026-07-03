-- name: InsertPlanStep :exec
INSERT INTO session_plans (id, session_id, step_index, description, status, blocker_reason, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeletePlanForSession :exec
DELETE FROM session_plans
WHERE session_id = ?;

-- name: ListPlanSteps :many
SELECT * FROM session_plans
WHERE session_id = ?
ORDER BY step_index ASC;

-- name: UpdatePlanStep :execrows
UPDATE session_plans
SET status = ?, blocker_reason = ?, updated_at = ?
WHERE session_id = ? AND step_index = ?;
