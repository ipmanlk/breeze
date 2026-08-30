-- name: UpsertPresence :exec
INSERT INTO user_presence (user_id, org_id, status, last_seen, updated_at)
VALUES (?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(user_id, org_id) DO UPDATE SET
    status = excluded.status,
    last_seen = excluded.last_seen,
    updated_at = excluded.updated_at;

-- name: GetPresence :one
SELECT * FROM user_presence WHERE user_id = ? AND org_id = ?;

-- name: GetPresenceForUsers :many
SELECT * FROM user_presence
WHERE org_id = ? AND user_id IN (sqlc.slice('user_ids'));

-- name: ListOrgPresence :many
SELECT up.*, u.name AS user_name, u.email AS user_email, u.avatar_url AS user_avatar_url
FROM user_presence up
INNER JOIN users u ON u.id = up.user_id
WHERE up.org_id = ?
ORDER BY
  CASE up.status WHEN 'online' THEN 0 WHEN 'away' THEN 1 ELSE 2 END,
  u.name ASC;

-- name: DeletePresence :execrows
DELETE FROM user_presence WHERE user_id = ? AND org_id = ?;
