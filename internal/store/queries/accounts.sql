-- name: CreateAccount :exec
INSERT INTO accounts (id, email, password_hash)
VALUES (?, ?, ?);

-- name: GetAccountByEmail :one
SELECT id, email, password_hash, created_at, updated_at
FROM accounts
WHERE email = ?;

-- name: GetAccountByID :one
SELECT id, email, password_hash, created_at, updated_at
FROM accounts
WHERE id = ?;

-- name: UpdateAccountPassword :exec
UPDATE accounts
SET password_hash = ?, updated_at = datetime('now')
WHERE id = ?;

-- name: AccountExists :one
SELECT COUNT(*) > 0 FROM accounts;
