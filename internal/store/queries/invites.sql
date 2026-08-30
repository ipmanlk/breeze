-- name: CreateUserInvite :exec
INSERT INTO user_invites (id, org_id, email, role, token_hash, invited_by, max_uses, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUserInviteByTokenHash :one
SELECT id, org_id, email, role, token_hash, invited_by, max_uses, use_count, expires_at, created_at
FROM user_invites
WHERE token_hash = ? AND expires_at > datetime('now')
  AND (max_uses IS NULL OR use_count < max_uses);

-- name: ListUserInvites :many
SELECT i.id, i.org_id, i.email, i.role, i.invited_by, i.use_count, i.expires_at, i.created_at,
       u.name as invited_by_name
FROM user_invites i
JOIN users u ON u.id = i.invited_by
WHERE i.org_id = @org_id
ORDER BY i.created_at DESC
LIMIT @limit_val;

-- name: IncrementInviteUseCount :exec
UPDATE user_invites SET use_count = use_count + 1 WHERE id = ?;

-- name: DeleteUserInvite :exec
DELETE FROM user_invites WHERE id = ? AND org_id = ?;

-- name: RecordInviteAcceptance :exec
INSERT INTO user_invite_acceptances (id, invite_id, user_id)
VALUES (?, ?, ?);
