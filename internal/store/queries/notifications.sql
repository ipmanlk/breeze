-- name: CreateNotification :exec
INSERT INTO notifications (id, org_id, user_id, type, title, body, link, entity_type, entity_id, actor_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListNotifications :many
SELECT n.id, n.org_id, n.user_id, n.type, n.title, n.body, n.link,
       n.entity_type, n.entity_id, n.actor_id,
       n.is_read, n.read_at, n.created_at,
       u.id AS actor_user_id, u.name AS actor_name, u.email AS actor_email, u.avatar_url AS actor_avatar_url,
       COALESCE(p.slug, '') AS project_slug
FROM notifications n
LEFT JOIN users u ON u.id = n.actor_id
LEFT JOIN tasks t ON t.id = n.entity_id AND n.entity_type = 'task'
LEFT JOIN projects p ON p.id = t.project_id
WHERE n.user_id = @user_id AND n.org_id = @org_id
  AND (@unread_only = 0 OR n.is_read = 0)
  AND (
    @cursor_created_at = '' OR @cursor_id = ''
    OR n.created_at < @cursor_created_at
    OR (n.created_at = @cursor_created_at AND n.id < @cursor_id)
  )
ORDER BY n.created_at DESC, n.id DESC
LIMIT @limit_val;

-- name: GetNotificationByID :one
SELECT n.id, n.org_id, n.user_id, n.type, n.title, n.body, n.link,
       n.entity_type, n.entity_id, n.actor_id,
       n.is_read, n.read_at, n.created_at,
       u.id AS actor_user_id, u.name AS actor_name, u.email AS actor_email, u.avatar_url AS actor_avatar_url,
       COALESCE(p.slug, '') AS project_slug
FROM notifications n
LEFT JOIN users u ON u.id = n.actor_id
LEFT JOIN tasks t ON t.id = n.entity_id AND n.entity_type = 'task'
LEFT JOIN projects p ON p.id = t.project_id
WHERE n.id = ? AND n.user_id = ?;

-- name: MarkNotificationRead :execrows
UPDATE notifications SET is_read = 1, read_at = datetime('now')
WHERE id = ? AND user_id = ?;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications SET is_read = 1, read_at = datetime('now')
WHERE user_id = ? AND is_read = 0;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0;

-- name: ListNotificationPreferences :many
SELECT type, enabled FROM notification_preferences WHERE user_id = ?;

-- name: GetNotificationPreferenceByType :one
SELECT type, enabled FROM notification_preferences WHERE user_id = ? AND type = ?;

-- name: SetNotificationPreference :exec
INSERT INTO notification_preferences (user_id, type, enabled)
VALUES (?, ?, ?)
ON CONFLICT(user_id, type) DO UPDATE SET enabled = excluded.enabled;

-- name: FindDueNotificationTasks :many
SELECT t.id AS task_id, t.title AS task_title, t.due_at, t.status_id,
       ta.user_id AS assignee_id, t.org_id, t.project_id,
       p.slug AS project_slug
FROM tasks t
JOIN task_assignees ta ON ta.task_id = t.id
JOIN projects p ON p.id = t.project_id
WHERE t.due_at IS NOT NULL
  AND t.completed_at IS NULL
  AND (
    (t.due_at < @now AND t.due_at >= @overdue_start)
    OR (t.due_at >= @now AND t.due_at < @due_soon_end)
  )
  AND NOT EXISTS (
    SELECT 1 FROM notifications n
    WHERE n.entity_type = 'task'
      AND n.entity_id = t.id
      AND n.user_id = ta.user_id
      AND n.type IN (@due_soon_type, @overdue_type)
      AND n.created_at > datetime('now', '-24 hours')
  );