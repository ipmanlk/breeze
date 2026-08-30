-- name: ListTasksByProject :many
SELECT t.id, t.org_id, t.project_id, t.cycle_id, t.parent_task_id, t.created_by, t.title, t.description, t.status_id, t.priority, t.position_key, t.estimate, t.started_at, t.due_at, t.completed_at, t.template_id, t.created_at, t.updated_at,
  (SELECT COUNT(*) FROM tasks c WHERE c.parent_task_id = t.id) AS subtask_count,
  (SELECT COUNT(*) FROM tasks c WHERE c.parent_task_id = t.id AND c.completed_at IS NOT NULL) AS completed_subtask_count
FROM tasks t
WHERE t.project_id = @project_id
  AND t.org_id = @org_id
  AND (@include_subtasks = 1 OR t.parent_task_id IS NULL)
  AND (@status_id IS NULL OR t.status_id = @status_id)
  AND (
    @cycle_id IS NULL
    OR (@cycle_id = '__backlog__' AND t.cycle_id IS NULL)
    OR (t.cycle_id = @cycle_id)
  )
  AND (@assignee_id IS NULL OR EXISTS (SELECT 1 FROM task_assignees WHERE task_assignees.task_id = t.id AND task_assignees.user_id = @assignee_id))
  AND (t.priority = @priority OR @priority = '')
  AND (instr(lower(t.title), lower(@search)) > 0 OR @search = '%%')
  AND (sqlc.narg('has_label_filter') = 0 OR EXISTS (
    SELECT 1 FROM task_labels tl
    WHERE tl.task_id = t.id AND tl.label_id IN (sqlc.slice('label_ids'))
  ))
ORDER BY t.position_key ASC;

-- name: GetTaskByID :one
SELECT t.id, t.org_id, t.project_id, t.cycle_id, t.parent_task_id, t.created_by, t.title, t.description, t.status_id, t.priority, t.position_key, t.subtask_position, t.estimate, t.started_at, t.due_at, t.completed_at, t.template_id, t.created_at, t.updated_at,
  (SELECT COUNT(*) FROM tasks c WHERE c.parent_task_id = t.id) AS subtask_count,
  (SELECT COUNT(*) FROM tasks c WHERE c.parent_task_id = t.id AND c.completed_at IS NOT NULL) AS completed_subtask_count,
  p.title AS parent_title
FROM tasks t
LEFT JOIN tasks p ON p.id = t.parent_task_id
WHERE t.id = ? AND t.project_id = ? AND t.org_id = ?;

-- name: ListSubtasks :many
-- Direct children of a task (one level deep). Ordered by subtask_position
-- (parent-scoped lexorank). Used by MoveToProject to recursively move
-- subtasks along with their parent, and by the dedicated
-- GET /tasks/{taskId}/subtasks endpoint.
SELECT id, org_id, project_id, cycle_id, parent_task_id, created_by, title, description, status_id, priority, position_key, subtask_position, estimate, started_at, due_at, completed_at, template_id, created_at, updated_at
FROM tasks
WHERE parent_task_id = ? AND org_id = ?
ORDER BY subtask_position ASC;

-- name: CreateTask :exec
INSERT INTO tasks (id, org_id, project_id, cycle_id, parent_task_id, created_by, title, description, status_id, priority, position_key, subtask_position, estimate, started_at, due_at, completed_at, template_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateTask :exec
UPDATE tasks
SET title = ?, description = ?, status_id = ?, priority = ?, cycle_id = ?, parent_task_id = ?, estimate = ?, started_at = ?, due_at = ?, completed_at = ?, position_key = ?, subtask_position = ?, updated_at = ?
WHERE id = ? AND project_id = ? AND org_id = ?;

-- name: UpdateTaskPosition :exec
UPDATE tasks
SET status_id = ?, position_key = ?, updated_at = datetime('now')
WHERE id = ? AND project_id = ? AND org_id = ?;

-- name: MoveTaskToProject :exec
-- Re-homes a task into a different project + status. Caller must validate
-- the target project/status belong to the org first. Clears cycle_id (cycles
-- are project-scoped) and parent_task_id (parent may not exist in the target
-- project). completed_at is passed explicitly so it stays in sync with the
-- target status category.
UPDATE tasks
SET project_id = ?, status_id = ?, cycle_id = NULL, parent_task_id = NULL,
    position_key = ?, completed_at = ?, updated_at = datetime('now')
WHERE id = ? AND project_id = ? AND org_id = ?;

-- name: DeleteTask :exec
DELETE FROM tasks
WHERE id = ? AND project_id = ? AND org_id = ?;

-- name: DeleteSubtasksByParent :exec
-- Deletes all direct children of a task (1-level nesting enforced by the
-- service, so no recursion needed). Used by cascade-delete.
DELETE FROM tasks
WHERE parent_task_id = ? AND org_id = ?;

-- name: PromoteSubtasksByParent :exec
-- Clears parent_task_id on all direct children so they become top-level
-- tasks. Used by the promote delete mode.
UPDATE tasks
SET parent_task_id = NULL, updated_at = datetime('now')
WHERE parent_task_id = ? AND org_id = ?;

-- name: CountSubtasksByParent :one
-- Returns the number of direct children of a task. Used by the delete handler
-- to decide whether to block (409) when no mode is specified.
SELECT COUNT(*) FROM tasks WHERE parent_task_id = ? AND org_id = ?;

-- name: GetLastSubtaskPosition :one
-- Returns the highest subtask_position among a task's children, or '' if none.
-- Used to generate a new lexorank key at the end when creating/reordering.
SELECT COALESCE(MAX(subtask_position), '') FROM tasks WHERE parent_task_id = ? AND org_id = ?;

-- name: UpdateSubtaskPosition :exec
-- Sets the subtask_position (parent-scoped ordering) for a single subtask.
-- Used by ReorderSubtasks to re-key children.
UPDATE tasks
SET subtask_position = ?, updated_at = datetime('now')
WHERE id = ? AND parent_task_id = ? AND org_id = ?;

-- name: GetLastPositionKey :one
SELECT COALESCE(MAX(position_key), '') FROM tasks WHERE project_id = ? AND status_id = ? AND org_id = ?;

-- name: GetPositionKeyNeighbors :one
SELECT
  COALESCE(
    (SELECT position_key FROM tasks t2
     WHERE t2.project_id = tasks.project_id
       AND t2.status_id  = tasks.status_id
       AND t2.position_key < tasks.position_key
     ORDER BY t2.position_key DESC LIMIT 1),
    ''
  ) AS prev_key,
  COALESCE(
    (SELECT position_key FROM tasks t3
     WHERE t3.project_id = tasks.project_id
       AND t3.status_id  = tasks.status_id
       AND t3.position_key > tasks.position_key
     ORDER BY t3.position_key ASC LIMIT 1),
    ''
  ) AS next_key
FROM tasks WHERE tasks.id = ? AND tasks.org_id = ? LIMIT 1;

-- name: ListTasksByCycle :many
SELECT id, org_id, project_id, cycle_id, parent_task_id, created_by, title, description, status_id, priority, position_key, estimate, started_at, due_at, completed_at, template_id, created_at, updated_at
FROM tasks
WHERE cycle_id = @cycle_id AND org_id = @org_id
ORDER BY position_key ASC;

-- name: UnassignCycleFromTasks :exec
UPDATE tasks SET cycle_id = NULL, updated_at = datetime('now') WHERE cycle_id = @cycle_id AND org_id = @org_id;

-- name: MoveTasksToCycle :exec
UPDATE tasks SET cycle_id = @to_cycle_id, updated_at = datetime('now') WHERE cycle_id = @from_cycle_id AND org_id = @org_id;

-- name: MoveIncompleteTasksToCycle :exec
UPDATE tasks SET cycle_id = @to_cycle_id, updated_at = datetime('now') WHERE cycle_id = @from_cycle_id AND org_id = @org_id AND completed_at IS NULL;

-- name: UnassignCycleFromIncompleteTasks :exec
UPDATE tasks SET cycle_id = NULL, updated_at = datetime('now') WHERE cycle_id = @cycle_id AND org_id = @org_id AND completed_at IS NULL;

-- name: GetTaskByIDAndOrg :one
SELECT id, org_id, project_id, cycle_id, parent_task_id, created_by, title, description, status_id, priority, position_key, estimate, started_at, due_at, completed_at, template_id, created_at, updated_at
FROM tasks
WHERE id = @id AND org_id = @org_id;

-- name: ListTasksByUser :many
SELECT t.id, t.org_id, t.project_id, t.cycle_id, t.parent_task_id, t.created_by,
       t.title, t.description, t.status_id, t.priority, t.position_key,
       t.estimate, t.started_at, t.due_at, t.completed_at, t.template_id, t.created_at, t.updated_at,
       p.name AS project_name, p.slug AS project_slug, p.color AS project_color,
       s.name AS status_name, s.color AS status_color
FROM tasks t
JOIN projects p ON p.id = t.project_id
JOIN task_statuses s ON s.id = t.status_id
WHERE t.org_id = @org_id
  AND EXISTS (
    SELECT 1 FROM task_assignees ta
    WHERE ta.task_id = t.id AND ta.user_id = @effective_assignee_id
  )
  AND (@status_id IS NULL OR t.status_id = @status_id)
  AND (t.priority = @priority OR @priority = '')
  AND (@show_completed = 1 OR t.completed_at IS NULL)
  AND (
    @cycle_id IS NULL
    OR (@cycle_id = '__backlog__' AND t.cycle_id IS NULL)
    OR (t.cycle_id = @cycle_id)
  )
  AND (@has_label_filter = 0 OR EXISTS (
    SELECT 1 FROM task_labels tl
    WHERE tl.task_id = t.id AND tl.label_id IN (sqlc.slice('label_ids'))
  ))
  AND (@require_project_membership = 0 OR EXISTS (
    SELECT 1 FROM project_members pm
    WHERE pm.project_id = t.project_id AND pm.user_id = @effective_assignee_id
  ))
  AND (instr(lower(t.title), lower(@search)) > 0 OR @search = '%%')
  AND (
    @cursor_updated_at = ''
    OR t.updated_at < @cursor_updated_at
    OR (t.updated_at = @cursor_updated_at AND t.id < @cursor_id)
  )
ORDER BY t.updated_at DESC, t.id DESC
LIMIT @limit_val;

-- name: CountTasksByUser :one
SELECT COUNT(*)
FROM tasks t
JOIN task_assignees ta ON ta.task_id = t.id AND ta.user_id = @user_id
WHERE t.org_id = @org_id
  AND (@status_id IS NULL OR t.status_id = @status_id)
  AND (t.priority = @priority OR @priority = '')
  AND (@show_completed = 1 OR t.completed_at IS NULL)
  AND (
    @cycle_id IS NULL
    OR (@cycle_id = '__backlog__' AND t.cycle_id IS NULL)
    OR (t.cycle_id = @cycle_id)
  )
  AND (instr(lower(t.title), lower(@search)) > 0 OR @search = '%%');

-- name: GetTasksByIDsFull :many
-- Full-projection batch fetch by IDs (org-scoped). Used by BatchUpdate which
-- needs complete task rows to update safely (priority/status/etc.). The
-- mention-hydration query GetTasksByIDs (mentions.sql) is intentionally
-- minimal; this is the full version for mutation paths.
SELECT id, org_id, project_id, cycle_id, parent_task_id, created_by, title,
       description, status_id, priority, estimate, position_key, subtask_position,
       template_id, started_at, due_at, completed_at, created_at, updated_at
FROM tasks
WHERE org_id = @org_id AND id IN (sqlc.slice('ids'));
