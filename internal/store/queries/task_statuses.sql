-- name: ListStatusesByProject :many
SELECT id, project_id, name, color, position, category, is_default
FROM task_statuses
WHERE project_id = ?
ORDER BY position ASC;

-- name: GetStatusByID :one
SELECT id, project_id, name, color, position, category, is_default
FROM task_statuses
WHERE id = ?;

-- name: CreateStatus :exec
INSERT INTO task_statuses (id, project_id, name, color, position, category, is_default)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateStatus :exec
UPDATE task_statuses
SET name = ?, color = ?, position = ?, category = ?
WHERE id = ? AND project_id = ?;

-- name: DeleteStatus :exec
DELETE FROM task_statuses
WHERE id = ? AND project_id = ?;

-- name: ReassignTasksOnStatusDelete :exec
UPDATE tasks
SET status_id = @new_status_id
WHERE status_id = @old_status_id AND project_id = @project_id;

-- name: CountTasksByStatus :one
SELECT COUNT(*) FROM tasks WHERE status_id = ? AND project_id = ?;

-- name: GetMaxStatusPosition :one
SELECT COALESCE(MAX(position), -1) FROM task_statuses WHERE project_id = ?;
