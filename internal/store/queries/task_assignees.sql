-- name: ListTaskAssignees :many
SELECT task_id, user_id FROM task_assignees WHERE task_id = ?;

-- name: ListAssigneesByTaskIDs :many
SELECT ta.task_id, u.id, u.name, u.email, u.avatar_url
FROM task_assignees ta
JOIN users u ON u.id = ta.user_id
WHERE ta.task_id IN (sqlc.slice('task_ids'));

-- name: SetTaskAssignees :exec
DELETE FROM task_assignees WHERE task_id = ?;

-- name: AddTaskAssignee :exec
INSERT INTO task_assignees (task_id, user_id) VALUES (?, ?);
