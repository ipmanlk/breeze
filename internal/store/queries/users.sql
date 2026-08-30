-- name: CreateUser :exec
INSERT INTO users (id, account_id, org_id, email, name, role, avatar_url, is_active)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE id = ? AND org_id = ?;

-- name: GetUserByEmail :one
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE org_id = ? AND email = ?;

-- name: GetUserByEmailAnyOrg :one
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE email = ?
LIMIT 1;

-- name: ListUsersByAccount :many
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE account_id = ?
ORDER BY updated_at DESC;

-- name: UpdateUserRole :exec
UPDATE users
SET role = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: UpdateUserActive :exec
UPDATE users
SET is_active = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: GetByOrgAndAccount :one
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE org_id = ? AND account_id = ?;

-- name: UpdateUser :exec
UPDATE users
SET name = ?, email = ?, avatar_url = ?, role = ?, is_active = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: UpdateAccountProfile :exec
-- Syncs the denormalized display columns (name, avatar_url) across ALL of an
-- account's memberships so every workspace sees the same identity.
UPDATE users
SET name = ?, avatar_url = ?, updated_at = datetime('now')
WHERE account_id = ?;

-- name: ListUsersPaginated :many
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE org_id = @org_id

  AND (@search IS NULL OR @search = '' OR instr(lower(name), lower(@search)) > 0)
  AND (@role IS NULL OR @role = '' OR role = @role)
  AND (@include_inactive = 1 OR is_active = 1)
  AND (
    (@cursor_name = '' AND @cursor_id = '')
    OR (name > @cursor_name)
    OR (name = @cursor_name AND id > @cursor_id)
  )
ORDER BY name ASC, id ASC
LIMIT @limit_val;

-- name: CountOwners :one
SELECT COUNT(*) as count FROM users WHERE org_id = ? AND role = 'owner' AND is_active = 1;

-- name: ListUsersByIDs :many
SELECT id, account_id, org_id, email, name, role, avatar_url, is_active, created_at, updated_at
FROM users
WHERE id IN (sqlc.slice('ids'));
