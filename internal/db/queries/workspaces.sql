-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, path)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetWorkspaceByPath :one
SELECT *
FROM workspaces
WHERE path = ?
LIMIT 1;

-- name: GetWorkspace :one
SELECT *
FROM workspaces
WHERE id = ?
LIMIT 1;
