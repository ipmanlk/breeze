-- name: CreateComment :exec
INSERT INTO comments (id, org_id, task_id, project_id, author_id, content, parent_id)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetCommentByID :one
SELECT c.id, c.org_id, c.task_id, c.project_id, c.author_id, c.content, c.parent_id,
       c.created_at, c.updated_at, c.edited_at, c.deleted_at,
       u.name AS author_name, u.email AS author_email, u.avatar_url AS author_avatar_url
FROM comments c
INNER JOIN users u ON u.id = c.author_id
WHERE c.id = ? AND c.org_id = ? AND c.deleted_at IS NULL;

-- name: ListCommentsByTask :many
-- Paginated: returns the newest @limit comments older than @before_cursor
-- (or the newest @limit if before_cursor is empty). Ordered ASC so the
-- thread reads top-to-bottom. Pass the oldest loaded created_at as the
-- next before_cursor to load the previous page.
-- Org-scoped (org_id) so a foreign-org task_id cannot be used to read comments.
SELECT c.id, c.org_id, c.task_id, c.project_id, c.author_id, c.content, c.parent_id,
       c.created_at, c.updated_at, c.edited_at, c.deleted_at,
       u.name AS author_name, u.email AS author_email, u.avatar_url AS author_avatar_url
FROM comments c
INNER JOIN users u ON u.id = c.author_id
WHERE c.task_id = ?
  AND c.org_id = ?
  AND c.deleted_at IS NULL
  AND (sqlc.narg('before_cursor') = '' OR c.created_at < sqlc.narg('before_cursor'))
ORDER BY c.created_at DESC
LIMIT @limit;

-- name: UpdateComment :exec
UPDATE comments
SET content = ?, updated_at = datetime('now'), edited_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: SoftDeleteComment :exec
UPDATE comments
SET deleted_at = datetime('now'), updated_at = datetime('now')
WHERE id = ? AND org_id = ?;
