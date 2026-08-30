-- name: CreatePasswordReset :exec
INSERT INTO password_resets (id, account_id, token_hash, expires_at)
VALUES (?, ?, ?, ?);

-- name: GetPasswordResetByTokenHash :one
SELECT id, account_id, token_hash, expires_at, used_at, created_at
FROM password_resets
WHERE token_hash = ?;

-- name: MarkPasswordResetUsed :execrows
-- Conditional update makes the token atomically single-use: two concurrent
-- confirms with the same valid token can't both pass (only one UPDATE wins).
UPDATE password_resets
SET used_at = datetime('now')
WHERE id = ? AND used_at IS NULL;

-- name: DeleteUsedAndExpiredPasswordResets :execrows
-- Cleanup for rows that no longer serve any purpose: consumed tokens and
-- expired-but-never-used ones.
DELETE FROM password_resets
WHERE used_at IS NOT NULL OR expires_at < datetime('now');
