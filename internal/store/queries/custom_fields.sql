-- name: CreateCustomField :exec
INSERT INTO custom_fields (id, org_id, project_id, name, field_type, options, position)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetCustomFieldByID :one
SELECT id, org_id, project_id, name, field_type, options, position, created_at, updated_at
FROM custom_fields
WHERE id = ? AND org_id = ?;

-- name: ListCustomFieldsByProject :many
SELECT id, org_id, project_id, name, field_type, options, position, created_at, updated_at
FROM custom_fields
WHERE project_id = ? AND org_id = ?
ORDER BY position ASC;

-- name: UpdateCustomField :exec
UPDATE custom_fields
SET name = ?, options = ?, position = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: DeleteCustomField :exec
DELETE FROM custom_fields WHERE id = ? AND org_id = ?;

-- name: SetTaskCustomFieldValue :exec
INSERT INTO task_custom_field_values (task_id, custom_field_id, value, updated_at)
VALUES (?, ?, ?, datetime('now'))
ON CONFLICT(task_id, custom_field_id) DO UPDATE SET value = excluded.value, updated_at = datetime('now');

-- name: GetTaskCustomFieldValues :many
SELECT task_id, custom_field_id, value, updated_at
FROM task_custom_field_values
WHERE task_id = ?;

-- name: ListTaskCustomFieldValuesByTaskIDs :many
SELECT task_id, custom_field_id, value, updated_at
FROM task_custom_field_values
WHERE task_id IN (sqlc.slice('task_ids'));

-- name: DeleteTaskCustomFieldValues :exec
DELETE FROM task_custom_field_values WHERE task_id = ?;
