-- name: CreateTaskTemplate :exec
INSERT INTO task_templates (id, org_id, project_id, name, description, priority, status_id, assignee_ids, estimate, recurrence_pattern, recurrence_days, next_run_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetTaskTemplateByID :one
SELECT *
FROM task_templates
WHERE id = ? AND org_id = ?;

-- name: ListTaskTemplatesByProject :many
SELECT *
FROM task_templates
WHERE project_id = ? AND org_id = ?
ORDER BY created_at DESC;

-- name: ListTaskTemplatesByOrg :many
SELECT *
FROM task_templates
WHERE org_id = ?
ORDER BY created_at DESC;

-- name: ListDueRecurringTemplates :many
SELECT *
FROM task_templates
WHERE recurrence_pattern != 'none'
  AND next_run_at IS NOT NULL
  AND next_run_at <= ?
ORDER BY next_run_at ASC;

-- name: UpdateTaskTemplate :exec
UPDATE task_templates
SET name = ?, description = ?, priority = ?, status_id = ?, assignee_ids = ?, estimate = ?, recurrence_pattern = ?, recurrence_days = ?, next_run_at = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: UpdateTaskTemplateNextRun :exec
UPDATE task_templates
SET next_run_at = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: DeleteTaskTemplate :exec
DELETE FROM task_templates WHERE id = ? AND org_id = ?;

-- name: ClaimDueRecurringTemplate :execrows
UPDATE task_templates
SET next_run_at = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ? AND next_run_at = ?;

-- name: SetTemplateLastError :exec
UPDATE task_templates
SET last_error = ?, last_error_at = datetime('now'), updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: ClearTemplateLastError :exec
UPDATE task_templates
SET last_error = NULL, last_error_at = NULL, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;
