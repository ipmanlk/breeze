-- name: AddTaskDependency :exec
INSERT INTO task_dependencies (task_id, blocks_task_id)
VALUES (?, ?);

-- name: RemoveTaskDependency :exec
DELETE FROM task_dependencies
WHERE task_id = ? AND blocks_task_id = ?;

-- name: ListBlockingTasks :many
-- Tasks that block the given task (the given task is blocked by these).
SELECT t.id, t.org_id, t.project_id, t.cycle_id, t.parent_task_id, t.created_by,
       t.title, t.description, t.status_id, t.priority, t.position_key,
       t.estimate, t.started_at, t.due_at, t.completed_at, t.created_at, t.updated_at
FROM task_dependencies d
JOIN tasks t ON t.id = d.blocks_task_id
WHERE d.task_id = ?
ORDER BY t.title ASC;

-- name: ListBlockedTasks :many
-- Tasks that the given task is blocking (these wait on the given task).
SELECT t.id, t.org_id, t.project_id, t.cycle_id, t.parent_task_id, t.created_by,
       t.title, t.description, t.status_id, t.priority, t.position_key,
       t.estimate, t.started_at, t.due_at, t.completed_at, t.created_at, t.updated_at
FROM task_dependencies d
JOIN tasks t ON t.id = d.task_id
WHERE d.blocks_task_id = ?
ORDER BY t.title ASC;
