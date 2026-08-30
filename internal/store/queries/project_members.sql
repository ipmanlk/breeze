-- name: ListProjectMembers :many
-- Only explicit project members (rows in project_members). Org owners/admins
-- have implicit access via EnsureProjectAccess but are not listed here, so the
-- list reflects exactly who can be added/removed. u.role is the org role
-- (used to decide whether the per-project role is meaningful/overridable);
-- pm.role is the project role (the effective role for project-scoped users).
SELECT u.id, u.name, u.email, u.avatar_url, u.role AS org_role, pm.role AS project_role
FROM users u
JOIN project_members pm ON pm.user_id = u.id AND pm.project_id = @project_id
WHERE u.org_id = @org_id
  AND (@search IS NULL OR @search = '' OR instr(lower(u.name), lower(@search)) > 0)
  AND (
    (@cursor_name = '' AND @cursor_id = '')
    OR (u.name > @cursor_name)
    OR (u.name = @cursor_name AND u.id > @cursor_id)
  )
ORDER BY u.name ASC, u.id ASC
LIMIT @limit_val;

-- name: GetProjectMember :one
SELECT u.id, u.name, u.email, u.avatar_url, u.role,
       COALESCE(pm.role, '') AS project_role
FROM users u
LEFT JOIN project_members pm ON pm.user_id = u.id AND pm.project_id = @project_id
WHERE u.id = @user_id AND u.org_id = @org_id;

-- name: AddProjectMember :exec
INSERT INTO project_members (project_id, user_id, role)
SELECT ?, u.id, ?
FROM users u
JOIN projects p ON p.id = ?
WHERE u.id = ? AND u.org_id = p.org_id;

-- name: RemoveProjectMember :exec
DELETE FROM project_members
WHERE project_id = ? AND user_id = ?;

-- name: UpdateProjectMemberRole :exec
UPDATE project_members
SET role = ?
WHERE project_id = ? AND user_id = ?;

-- name: ListUserProjectMemberships :many
SELECT pm.project_id, p.name, p.color, p.icon, pm.role
FROM project_members pm
JOIN projects p ON p.id = pm.project_id
WHERE pm.user_id = ? AND p.org_id = ?
ORDER BY p.name ASC;
