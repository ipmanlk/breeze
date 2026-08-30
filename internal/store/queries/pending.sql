-- name: CreatePendingAttachment :exec
INSERT INTO pending_attachments (id, conversation_id, file_name, file_size, content_type, storage_path, uploaded_by)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetPendingAttachmentByID :one
SELECT * FROM pending_attachments WHERE id = ? AND uploaded_by = ?;

-- name: DeletePendingAttachment :exec
DELETE FROM pending_attachments WHERE id = ?;

-- name: DeletePendingAttachmentsOlderThan :many
DELETE FROM pending_attachments WHERE created_at < ?
RETURNING *;
