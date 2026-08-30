-- name: UpsertChannelPreference :exec
INSERT INTO user_channel_preferences (user_id, conversation_id, org_id, notification_level, muted, last_read_at, updated_at)
VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(user_id, conversation_id) DO UPDATE SET
    notification_level = excluded.notification_level,
    muted = excluded.muted,
    updated_at = excluded.updated_at;

-- name: UpsertChannelMute :exec
INSERT INTO user_channel_preferences (user_id, conversation_id, org_id, muted, updated_at)
VALUES (?, ?, ?, ?, datetime('now'))
ON CONFLICT(user_id, conversation_id) DO UPDATE SET
    muted = excluded.muted,
    updated_at = excluded.updated_at;

-- name: UpsertChannelNotificationLevel :exec
INSERT INTO user_channel_preferences (user_id, conversation_id, org_id, notification_level, updated_at)
VALUES (?, ?, ?, ?, datetime('now'))
ON CONFLICT(user_id, conversation_id) DO UPDATE SET
    notification_level = excluded.notification_level,
    updated_at = excluded.updated_at;

-- name: GetChannelPreference :one
SELECT * FROM user_channel_preferences
WHERE user_id = ? AND conversation_id = ?;

-- name: GetNotificationLevel :one
SELECT notification_level FROM user_channel_preferences
WHERE user_id = ? AND conversation_id = ?;

-- name: UpdateChannelLastRead :exec
UPDATE user_channel_preferences SET last_read_at = datetime('now'), updated_at = datetime('now')
WHERE user_id = ? AND conversation_id = ?;

-- name: ListChannelsForUser :many
SELECT * FROM user_channel_preferences
WHERE user_id = ?
ORDER BY conversation_id;
