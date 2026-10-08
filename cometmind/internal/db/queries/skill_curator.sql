-- name: GetSkillCuratorPass :one
SELECT *
FROM skill_curator_pass
WHERE id = 1;

-- name: EnsureSkillCuratorPass :exec
INSERT INTO skill_curator_pass (id, last_pass_at, last_merge_at, runs_idle_since)
VALUES (1, 0, 0, 0)
ON CONFLICT (id) DO NOTHING;

-- name: UpdateSkillCuratorPass :exec
UPDATE skill_curator_pass
SET
    last_pass_at = ?,
    last_merge_at = ?,
    runs_idle_since = ?
WHERE id = 1;

-- name: SetCuratorRunsIdleSince :exec
UPDATE skill_curator_pass
SET runs_idle_since = ?
WHERE id = 1;

-- name: ListSkillCuratorStates :many
SELECT *
FROM skill_curator_state
ORDER BY skill_name;

-- name: GetSkillCuratorState :one
SELECT *
FROM skill_curator_state
WHERE skill_name = ?;

-- name: UpsertSkillCuratorState :exec
INSERT INTO skill_curator_state (
    skill_name,
    origin,
    status,
    pinned,
    created_at,
    last_used_at,
    unused_since,
    archived_at,
    pinned_at,
    delete_notified_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (skill_name) DO UPDATE
SET
    status = excluded.status,
    pinned = excluded.pinned,
    last_used_at = excluded.last_used_at,
    unused_since = excluded.unused_since,
    archived_at = excluded.archived_at,
    pinned_at = excluded.pinned_at,
    delete_notified_at = excluded.delete_notified_at;

-- name: DeleteSkillCuratorState :exec
DELETE FROM skill_curator_state
WHERE skill_name = ?;

-- name: CountSessionRuns :one
SELECT count(*)
FROM session_runs;
