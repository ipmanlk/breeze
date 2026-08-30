-- name: CreateTaskActivity :exec
INSERT INTO task_activity (id, task_id, org_id, project_id, actor_id, action, field, old_value, new_value)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListTaskActivity :many
SELECT ta.id, ta.task_id, ta.org_id, ta.project_id, ta.actor_id, ta.action,
       ta.field, ta.old_value, ta.new_value, ta.created_at,
       u.name AS actor_name, u.email AS actor_email
FROM task_activity ta
JOIN users u ON u.id = ta.actor_id
WHERE ta.task_id = @task_id
  AND (
    (@cursor_created_at = '' AND @cursor_id = '')
    OR (ta.created_at < @cursor_created_at)
    OR (ta.created_at = @cursor_created_at AND ta.id < @cursor_id)
  )
ORDER BY ta.created_at DESC, ta.id DESC
LIMIT @limit_val;
