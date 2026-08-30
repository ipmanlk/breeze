-- name: MyTasks :many
SELECT t.id, t.title, t.priority, t.status_id, ts.name AS status_name, ts.color AS status_color,
       t.project_id, p.name AS project_name, p.slug AS project_slug, t.due_at
FROM tasks t
JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
JOIN task_statuses ts ON ts.id = t.status_id
WHERE t.org_id = @org_id
  AND t.completed_at IS NULL
ORDER BY CASE WHEN t.priority = 'urgent' THEN 0 WHEN t.priority = 'high' THEN 1 WHEN t.priority = 'medium' THEN 2 ELSE 3 END,
         t.due_at ASC NULLS LAST,
         t.updated_at DESC
LIMIT @limit;

-- name: DueSoonTasks :many
SELECT t.id, t.title, t.priority, t.status_id, ts.name AS status_name, ts.color AS status_color,
       t.project_id, p.name AS project_name, p.slug AS project_slug, t.due_at
FROM tasks t
JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
JOIN task_statuses ts ON ts.id = t.status_id
WHERE t.org_id = @org_id
  AND t.due_at IS NOT NULL
  AND t.completed_at IS NULL
ORDER BY t.due_at ASC
LIMIT @limit;

-- name: MyTaskStats :one
SELECT
  (SELECT COUNT(*) FROM tasks t
   JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
   WHERE t.org_id = @org_id AND t.completed_at IS NULL) AS assigned_count,
  (SELECT COUNT(*) FROM tasks t
   JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
   WHERE t.org_id = @org_id AND t.due_at IS NOT NULL AND t.due_at < datetime('now') AND t.completed_at IS NULL) AS overdue_count,
  (SELECT COUNT(*) FROM tasks t
   JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
   WHERE t.org_id = @org_id AND t.due_at IS NOT NULL AND t.due_at >= datetime('now') AND t.due_at < datetime('now', '+7 days') AND t.completed_at IS NULL) AS due_this_week_count,
  (SELECT COUNT(*) FROM tasks t
   JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
   WHERE t.org_id = @org_id AND t.completed_at IS NOT NULL) AS completed_count,
  (SELECT COUNT(*) FROM projects WHERE org_id = @org_id) AS total_projects;

-- name: RecentActivity :many
SELECT n.id, n.type, n.title, n.body, n.link,
       n.entity_type, n.entity_id, n.is_read, n.created_at,
       COALESCE(u.name, '') AS actor_name,
       COALESCE(p.slug, '') AS project_slug
FROM notifications n
LEFT JOIN users u ON u.id = n.actor_id
LEFT JOIN tasks t ON t.id = n.entity_id AND n.entity_type = 'task'
LEFT JOIN projects p ON p.id = t.project_id
WHERE n.user_id = @user_id AND n.org_id = @org_id
ORDER BY n.created_at DESC
LIMIT @limit;

-- name: OrgProjects :many
SELECT p.id, p.name, p.slug, p.color, p.icon,
       (SELECT COUNT(*) FROM tasks t WHERE t.project_id = p.id AND t.completed_at IS NULL) AS task_count,
       (SELECT COUNT(*) FROM project_members pm WHERE pm.project_id = p.id) AS member_count
FROM projects p
WHERE p.org_id = @org_id
ORDER BY p.updated_at DESC;

-- name: OrgProjectsForUser :many
-- Projects the caller is an explicit member of. Used for the dashboard
-- "projects" section when the caller is a viewer/guest (project-scoped role).
SELECT p.id, p.name, p.slug, p.color, p.icon,
       (SELECT COUNT(*) FROM tasks t WHERE t.project_id = p.id AND t.completed_at IS NULL) AS task_count,
       (SELECT COUNT(*) FROM project_members pm2 WHERE pm2.project_id = p.id) AS member_count
FROM projects p
JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE p.org_id = @org_id
ORDER BY p.updated_at DESC;

-- name: GetDashboardPreferences :one
SELECT sections FROM dashboard_preferences
WHERE user_id = @user_id AND org_id = @org_id;

-- name: SetDashboardPreferences :exec
INSERT INTO dashboard_preferences (user_id, org_id, sections)
VALUES (@user_id, @org_id, @sections)
ON CONFLICT(user_id, org_id) DO UPDATE SET sections = excluded.sections;
