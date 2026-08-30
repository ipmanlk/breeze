-- name: CreateOrganization :exec
INSERT INTO organizations (id, name, slug)
VALUES (?, ?, ?);

-- name: GetOrganizationByID :one
SELECT id, name, slug, message_edit_window_minutes, created_at, updated_at
FROM organizations
WHERE id = ?;

-- name: GetOrganizationBySlug :one
SELECT id, name, slug, message_edit_window_minutes, created_at, updated_at
FROM organizations
WHERE slug = ?;

-- name: OrganizationExists :one
SELECT COUNT(*) > 0 FROM organizations;

-- name: OrganizationCount :one
SELECT COUNT(*) FROM organizations;

-- name: UpdateOrganization :exec
UPDATE organizations
SET name = ?, slug = ?, message_edit_window_minutes = ?, updated_at = datetime('now')
WHERE id = ?;

-- name: DeleteOrganization :exec
DELETE FROM organizations WHERE id = ?;

-- name: ListOrganizationsByAccount :many
SELECT o.id, o.name, o.slug, o.message_edit_window_minutes, o.created_at, o.updated_at,
       u.role AS user_role, u.is_active AS user_is_active
FROM organizations o
INNER JOIN users u ON u.org_id = o.id
WHERE u.account_id = ?
ORDER BY u.updated_at DESC;
