-- name: ListProjectsByOrg :many
SELECT id, org_id, name, description, slug, color, icon, created_by, cycle_duration, auto_generate_cycles, incomplete_task_handling, starts_at, ends_at, is_archived, created_at, updated_at
FROM projects
WHERE org_id = @org_id AND (@include_archived = 1 OR is_archived = FALSE)
ORDER BY created_at DESC;

-- name: ListProjectsByMembership :many
-- Projects a project-scoped user (viewer/guest) is an explicit member of.
SELECT p.id, p.org_id, p.name, p.description, p.slug, p.color, p.icon, p.created_by, p.cycle_duration, p.auto_generate_cycles, p.incomplete_task_handling, p.starts_at, p.ends_at, p.is_archived, p.created_at, p.updated_at
FROM projects p
JOIN project_members pm ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE p.org_id = @org_id AND (@include_archived = 1 OR p.is_archived = FALSE)
ORDER BY p.created_at DESC;

-- name: GetProjectByID :one
SELECT id, org_id, name, description, slug, color, icon, created_by, cycle_duration, auto_generate_cycles, incomplete_task_handling, starts_at, ends_at, is_archived, created_at, updated_at
FROM projects
WHERE id = @id AND org_id = @org_id;

-- name: GetProjectBySlug :one
SELECT id, org_id, name, description, slug, color, icon, created_by, cycle_duration, auto_generate_cycles, incomplete_task_handling, starts_at, ends_at, is_archived, created_at, updated_at
FROM projects
WHERE org_id = @org_id AND slug = @slug;

-- name: CreateProject :exec
INSERT INTO projects (id, org_id, name, description, slug, color, icon, created_by, cycle_duration, auto_generate_cycles, incomplete_task_handling, starts_at, ends_at)
VALUES (@id, @org_id, @name, @description, @slug, @color, @icon, @created_by, @cycle_duration, @auto_generate_cycles, @incomplete_task_handling, @starts_at, @ends_at);

-- name: UpdateProject :exec
UPDATE projects
SET name = @name, description = @description, color = @color, icon = @icon, cycle_duration = @cycle_duration, auto_generate_cycles = @auto_generate_cycles, incomplete_task_handling = @incomplete_task_handling, starts_at = @starts_at, ends_at = @ends_at, updated_at = datetime('now')
WHERE id = @id AND org_id = @org_id;

-- name: SetProjectArchived :exec
UPDATE projects
SET is_archived = @is_archived, updated_at = datetime('now')
WHERE id = @id AND org_id = @org_id;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = @id AND org_id = @org_id;
