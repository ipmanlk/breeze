-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, org_id, role, expires_at, user_agent, ip_address)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetSessionByID :one
SELECT id, user_id, org_id, role, expires_at, revoked_at, created_at, user_agent, ip_address
FROM sessions
WHERE id = ?;

-- name: ListSessionsByUser :many
-- Active + revoked sessions for the user, most recent first. Expired rows
-- are excluded so the list only shows sessions that could still be valid
-- (or were explicitly revoked within their lifetime).
SELECT id, user_id, org_id, role, expires_at, revoked_at, created_at, user_agent, ip_address
FROM sessions
WHERE user_id = ? AND expires_at > datetime('now')
ORDER BY created_at DESC;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = datetime('now') WHERE id = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= datetime('now');

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = ?;
