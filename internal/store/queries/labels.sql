-- name: CreateLabel :exec
INSERT INTO labels (id, org_id, name, color)
VALUES (?, ?, ?, ?);

-- name: GetLabelByID :one
SELECT id, org_id, name, color, created_at, updated_at
FROM labels
WHERE id = ? AND org_id = ?;

-- name: ListLabelsByOrg :many
SELECT id, org_id, name, color, created_at, updated_at
FROM labels
WHERE org_id = ?
ORDER BY name ASC;

-- name: UpdateLabel :exec
UPDATE labels
SET name = ?, color = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: DeleteLabel :exec
DELETE FROM labels WHERE id = ? AND org_id = ?;

-- name: ClearTaskLabels :exec
DELETE FROM task_labels WHERE task_id = ?;

-- name: AddTaskLabel :exec
INSERT INTO task_labels (task_id, label_id)
VALUES (?, ?);

-- name: GetTaskLabels :many
SELECT l.id, l.org_id, l.name, l.color, l.created_at, l.updated_at
FROM labels l
INNER JOIN task_labels tl ON tl.label_id = l.id
WHERE tl.task_id = ?
ORDER BY l.name ASC;

-- name: ListLabelsByTaskIDs :many
SELECT tl.task_id, l.id, l.org_id, l.name, l.color, l.created_at, l.updated_at
FROM task_labels tl
INNER JOIN labels l ON l.id = tl.label_id
WHERE tl.task_id IN (sqlc.slice('task_ids'))
ORDER BY l.name ASC;
