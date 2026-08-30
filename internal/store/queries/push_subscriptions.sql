-- name: CreatePushSubscription :one
INSERT INTO push_subscriptions (id, user_id, org_id, endpoint, p256dh, auth_key)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh = excluded.p256dh, auth_key = excluded.auth_key
RETURNING id, user_id, org_id, endpoint, p256dh, auth_key, created_at;

-- name: ListPushSubscriptionsByUser :many
SELECT id, user_id, org_id, endpoint, p256dh, auth_key, created_at
FROM push_subscriptions
WHERE user_id = ?;

-- name: DeletePushSubscription :execrows
DELETE FROM push_subscriptions
WHERE user_id = ? AND endpoint = ?;

-- name: DeletePushSubscriptionsByUser :execrows
DELETE FROM push_subscriptions
WHERE user_id = ?;
