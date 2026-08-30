-- name: ListCyclesByProject :many
SELECT id, org_id, project_id, name, goal, starts_at, ends_at, created_by, is_completed, is_active, created_at, updated_at
FROM cycles
WHERE project_id = @project_id
ORDER BY starts_at ASC;

-- name: GetCycleByID :one
SELECT id, org_id, project_id, name, goal, starts_at, ends_at, created_by, is_completed, is_active, created_at, updated_at
FROM cycles
WHERE id = @id AND project_id = @project_id;

-- name: GetActiveCycleByProject :one
SELECT id, org_id, project_id, name, goal, starts_at, ends_at, created_by, is_completed, is_active, created_at, updated_at
FROM cycles
WHERE project_id = @project_id AND is_active = TRUE;

-- name: CreateCycle :exec
INSERT INTO cycles (id, org_id, project_id, name, goal, starts_at, ends_at, created_by)
VALUES (@id, @org_id, @project_id, @name, @goal, @starts_at, @ends_at, @created_by);

-- name: UpdateCycle :exec
UPDATE cycles
SET name = @name, goal = @goal, starts_at = @starts_at, ends_at = @ends_at, is_completed = @is_completed, is_active = @is_active, updated_at = datetime('now')
WHERE id = @id AND project_id = @project_id;

-- name: DeleteCycle :exec
DELETE FROM cycles
WHERE id = @id AND project_id = @project_id;

-- name: CountCyclesByProject :one
SELECT COUNT(*) FROM cycles WHERE project_id = @project_id;

-- name: DeactivateAllCycles :exec
UPDATE cycles SET is_active = FALSE, updated_at = datetime('now') WHERE project_id = @project_id;

-- name: SetCycleActive :exec
UPDATE cycles SET is_active = TRUE, updated_at = datetime('now') WHERE id = @id AND project_id = @project_id;

-- name: CountTasksByCycle :one
SELECT
  COUNT(*) AS total,
  SUM(CASE WHEN completed_at IS NOT NULL THEN 1 ELSE 0 END) AS completed
FROM tasks
WHERE cycle_id = @cycle_id;

-- name: CountTasksByCycles :many
SELECT
  c.id AS cycle_id,
  COUNT(t.id) AS total,
  SUM(CASE WHEN t.completed_at IS NOT NULL THEN 1 ELSE 0 END) AS completed
FROM cycles c
LEFT JOIN tasks t ON t.cycle_id = c.id
WHERE c.project_id = @project_id
GROUP BY c.id;