-- name: InsertMemory :exec
INSERT INTO memories (
    id,
    scope,
    kind,
    preference_category,
    content,
    embedding,
    embedding_model,
    source,
    base_weight,
    access_count,
    pinned,
    source_session_id,
    superseded_by,
    archived,
    archived_reason,
    last_accessed_at,
    created_at,
    updated_at
) VALUES (
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?,
    ?
);

-- name: UpdateMemory :exec
UPDATE memories
SET
    kind = ?,
    preference_category = ?,
    content = ?,
    embedding = ?,
    embedding_model = ?,
    base_weight = ?,
    pinned = ?,
    last_accessed_at = ?,
    updated_at = ?
WHERE id = ?;

-- name: ArchiveMemory :exec
UPDATE memories
SET
    archived = 1,
    archived_reason = ?,
    superseded_by = ?,
    updated_at = ?
WHERE id = ?;

-- name: TouchMemoryAccess :exec
UPDATE memories
SET
    access_count = access_count + 1,
    last_accessed_at = ?,
    updated_at = ?
WHERE id = ?;

-- name: GetMemory :one
SELECT *
FROM memories
WHERE id = ?;

-- name: ListActiveMemories :many
SELECT *
FROM memories
WHERE archived = 0
ORDER BY created_at DESC;

-- name: ListBaselinePreferences :many
SELECT *
FROM memories
WHERE archived = 0
  AND kind = 'preference'
ORDER BY pinned DESC, updated_at DESC, base_weight DESC, access_count DESC
LIMIT ?;

-- name: ListActivePreferencesByCategory :many
SELECT *
FROM memories
WHERE archived = 0
  AND kind = 'preference'
  AND preference_category = ?
ORDER BY pinned DESC, updated_at DESC, base_weight DESC, access_count DESC;

-- name: CountActiveMemories :one
SELECT COUNT(*)
FROM memories
WHERE archived = 0;

-- name: DeleteMemory :exec
DELETE FROM memories
WHERE id = ?;

-- name: ListArchivedMemoryIDsOlderThan :many
SELECT id
FROM memories
WHERE archived = 1 AND updated_at < ?
ORDER BY updated_at ASC;

-- name: DeleteMemoryEventsByMemoryID :exec
DELETE FROM memory_events
WHERE memory_id = ?;

-- name: DeleteMemoryEventsOlderThan :exec
DELETE FROM memory_events
WHERE created_at < ?;

-- name: InsertMemoryEvent :exec
INSERT INTO memory_events (id, memory_id, action, detail, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListMemoryEvents :many
SELECT *
FROM memory_events
ORDER BY created_at DESC
LIMIT ?;
