-- name: ListAttachments :many
SELECT id, task_id, filename, content_type, size, storage_path, created_by, created_at
FROM attachments WHERE task_id = ? ORDER BY created_at DESC;

-- name: GetAttachment :one
SELECT id, task_id, filename, content_type, size, storage_path, created_by, created_at
FROM attachments WHERE id = ?;

-- name: CreateAttachment :exec
INSERT INTO attachments (id, task_id, filename, content_type, size, storage_path, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteAttachment :exec
DELETE FROM attachments WHERE id = ? AND task_id = ?;
