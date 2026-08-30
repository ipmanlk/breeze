-- name: SearchProjects :many
SELECT p.id, p.name, p.slug, p.color
FROM projects_fts f
JOIN projects p ON p.id = f.project_id
WHERE f.org_id = @org_id
  AND f.name MATCH sqlc.arg('query')
  AND (sqlc.narg('project_id') IS NULL OR p.id = sqlc.narg('project_id'))
ORDER BY p.name ASC
LIMIT @limit;

-- name: SearchProjectsForUser :many
-- Project search scoped to projects the caller is an explicit member of.
-- Used for viewer/guest users so they only discover projects they can access.
SELECT p.id, p.name, p.slug, p.color
FROM projects_fts f
JOIN projects p ON p.id = f.project_id AND p.org_id = @org_id
JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE f.org_id = @org_id
  AND f.name MATCH sqlc.arg('query')
ORDER BY p.name ASC
LIMIT @limit;

-- name: SearchTasks :many
SELECT t.id, t.title, t.project_id, p.name AS project_name, p.slug AS project_slug, p.color AS project_color
FROM tasks_fts f
JOIN tasks t ON t.id = f.task_id
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
WHERE f.org_id = @org_id
  AND f.title MATCH sqlc.arg('query')
  AND (sqlc.narg('project_id') IS NULL OR t.project_id = sqlc.narg('project_id'))
ORDER BY t.title ASC
LIMIT @limit;

-- name: SearchTasksForUser :many
-- Task search scoped to projects the caller is an explicit member of. Used
-- for viewer/guest users (project-scoped roles) so they only discover tasks in
-- projects they can access. Elevated org roles (owner/admin/member) use the
-- org-wide SearchTasks query instead.
SELECT t.id, t.title, t.project_id, p.name AS project_name, p.slug AS project_slug, p.color AS project_color
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
JOIN project_members pm ON pm.project_id = t.project_id AND pm.user_id = @user_id
WHERE t.org_id = @org_id
  AND t.id IN (SELECT task_id FROM tasks_fts WHERE tasks_fts.title MATCH sqlc.arg('query'))
  AND (sqlc.narg('project_id') IS NULL OR t.project_id = sqlc.narg('project_id'))
ORDER BY t.title ASC
LIMIT @limit;

-- name: SearchChannels :many
WITH RECURSIVE conv_with_link AS (
  SELECT c.id AS conv_id
  FROM conversations c
  WHERE EXISTS (SELECT 1 FROM channel_project_links cpl WHERE cpl.channel_id = c.id)
    AND c.org_id = @org_id AND c.deleted_at IS NULL
  UNION ALL
  SELECT c.id
  FROM conversations c
  JOIN conv_with_link p ON c.parent_id = p.conv_id
  WHERE c.org_id = @org_id AND c.deleted_at IS NULL
)
SELECT c.id, c.name
FROM conversations c
WHERE c.org_id = @org_id
  AND c.type = 'channel'
  AND c.deleted_at IS NULL
  AND instr(lower(c.name), lower(@query)) > 0
  AND (
    EXISTS (
      SELECT 1 FROM conversation_members cm
      WHERE cm.conversation_id = c.id AND cm.user_id = @user_id
    )
    OR (@include_project_linked = 1 AND c.id IN (SELECT conv_id FROM conv_with_link))
  )
ORDER BY c.name ASC
LIMIT @limit;

-- name: SearchDirectMessages :many
-- DM search only returns conversations the caller is a member of; the
-- partner join is for the display name, not for discovery.
SELECT c.id, u.name AS partner_name
FROM conversations c
JOIN conversation_members me ON me.conversation_id = c.id AND me.user_id = @user_id
JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id != @user_id
JOIN users u ON u.id = cm.user_id
WHERE c.org_id = @org_id
  AND c.type = 'direct'
  AND (
    instr(lower(u.name), lower(@query)) > 0
    OR EXISTS (
      SELECT 1 FROM conversation_members cm2
      JOIN users u2 ON u2.id = cm2.user_id
      WHERE cm2.conversation_id = c.id AND instr(lower(u2.name), lower(@query)) > 0
    )
  )
ORDER BY u.name ASC
LIMIT @limit;

-- name: SearchMembers :many
SELECT id, name, avatar_url
FROM users
WHERE org_id = @org_id
  AND instr(lower(name), lower(@query)) > 0
ORDER BY name ASC
LIMIT @limit;

-- name: RecentProjects :many
SELECT id, name, slug, color, updated_at
FROM projects
WHERE org_id = @org_id
ORDER BY updated_at DESC
LIMIT @limit;

-- name: RecentProjectsForUser :many
-- Recent projects scoped to projects the caller is an explicit member of.
-- Used for viewer/guest users.
SELECT p.id, p.name, p.slug, p.color, p.updated_at
FROM projects p
JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE p.org_id = @org_id
ORDER BY p.updated_at DESC
LIMIT @limit;

-- name: RecentTasks :many
SELECT t.id, t.title, t.project_id, p.name AS project_name, p.slug AS project_slug, p.color AS project_color, t.updated_at
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
WHERE t.org_id = @org_id
ORDER BY t.updated_at DESC
LIMIT @limit;

-- name: RecentTasksForUser :many
SELECT t.id, t.title, t.project_id, p.name AS project_name, p.slug AS project_slug, p.color AS project_color, t.updated_at
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
JOIN project_members pm ON pm.project_id = t.project_id AND pm.user_id = @user_id
WHERE t.org_id = @org_id
ORDER BY t.updated_at DESC
LIMIT @limit;
