-- name: SearchMentionUsers :many
-- Search users by name within an org. Used for @mention autocomplete.
SELECT id, name, avatar_url
FROM users
WHERE org_id = @org_id
  AND is_active = 1
  AND (@query = '' OR instr(lower(name), lower(@query)) > 0)
ORDER BY name ASC
LIMIT @limit_val;

-- name: SearchMentionProjects :many
-- Search projects accessible to the requesting user (for privileged roles: owner/admin/member).
SELECT p.id, p.name, p.icon, p.color
FROM projects p
WHERE p.org_id = @org_id
  AND (@query = '' OR instr(lower(p.name), lower(@query)) > 0)
ORDER BY p.name ASC
LIMIT @limit_val;

-- name: SearchMentionProjectsForUser :many
-- Search projects the user is explicitly a member of (for viewer role).
SELECT p.id, p.name, p.icon, p.color
FROM projects p
INNER JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE p.org_id = @org_id
  AND (@query = '' OR instr(lower(p.name), lower(@query)) > 0)
ORDER BY p.name ASC
LIMIT @limit_val;

-- name: SearchMentionTasks :many
-- Search tasks by title across all projects in org (for privileged roles: owner/admin/member).
SELECT t.id, t.title, t.project_id, p.name AS project_name
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
WHERE t.org_id = @org_id
  AND (@query = '' OR instr(lower(t.title), lower(@query)) > 0)
ORDER BY t.updated_at DESC
LIMIT @limit_val;

-- name: SearchMentionTasksForUser :many
-- Search tasks in projects the user is a member of (for viewer role).
SELECT t.id, t.title, t.project_id, p.name AS project_name
FROM tasks t
JOIN projects p ON p.id = t.project_id AND p.org_id = @org_id
INNER JOIN project_members pm ON pm.project_id = t.project_id AND pm.user_id = @user_id
WHERE t.org_id = @org_id
  AND (@query = '' OR instr(lower(t.title), lower(@query)) > 0)
ORDER BY t.updated_at DESC
LIMIT @limit_val;

-- name: SearchMentionChannels :many
-- Search channels (type=channel) accessible to the requesting user.
SELECT c.id, c.name
FROM conversations c
INNER JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = @user_id
WHERE c.org_id = @org_id
  AND c.type = 'channel'
  AND c.deleted_at IS NULL
  AND (@query = '' OR instr(lower(c.name), lower(@query)) > 0)
ORDER BY c.name ASC
LIMIT @limit_val;

-- name: GetTasksByIDs :many
-- Batch fetch tasks by IDs for mention hydration. Org-scoped so foreign-org
-- IDs simply don't match (no Go post-filter needed).
SELECT id, title, project_id, org_id
FROM tasks
WHERE org_id = @org_id AND id IN (sqlc.slice('ids'));

-- name: GetProjectsByIDs :many
-- Batch fetch projects by IDs for mention hydration.
SELECT id, name, org_id, icon, color
FROM projects
WHERE id IN (sqlc.slice('ids'));
