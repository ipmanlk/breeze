-- name: GetUserPreferences :one
SELECT * FROM user_preferences WHERE user_id = ?;

-- name: UpsertUserPreferences :exec
INSERT INTO user_preferences (
    user_id, theme, language, timezone,
    email_notifications, desktop_notifications, notification_sounds,
    sidebar_collapsed, motion_settings
) VALUES (
    @user_id, @theme, @language, @timezone,
    @email_notifications, @desktop_notifications, @notification_sounds,
    @sidebar_collapsed, @motion_settings
)
ON CONFLICT(user_id) DO UPDATE SET
    theme                = excluded.theme,
    language             = excluded.language,
    timezone             = excluded.timezone,
    email_notifications  = excluded.email_notifications,
    desktop_notifications = excluded.desktop_notifications,
    notification_sounds  = excluded.notification_sounds,
    sidebar_collapsed    = excluded.sidebar_collapsed,
    motion_settings      = excluded.motion_settings,
    updated_at           = datetime('now');
