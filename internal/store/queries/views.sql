-- name: CreateView :exec
INSERT INTO views (id, org_id, project_id, created_by, name, layout, filters)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateView :exec
UPDATE views
SET name = ?,
    layout = ?,
    filters = ?,
    updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: DeleteView :exec
DELETE FROM views WHERE id = ? AND org_id = ?;

-- name: GetViewByID :one
SELECT id, org_id, project_id, created_by, name, layout, filters, created_at, updated_at
FROM views
WHERE id = ? AND org_id = ?;

-- name: ListViewsByProject :many
SELECT id, org_id, project_id, created_by, name, layout, filters, created_at, updated_at
FROM views
WHERE org_id = ?1
  AND project_id = ?2
ORDER BY created_at ASC;

-- name: ListGlobalViews :many
SELECT id, org_id, project_id, created_by, name, layout, filters, created_at, updated_at
FROM views
WHERE org_id = ?
  AND project_id IS NULL
ORDER BY created_at ASC;

-- name: ListPinnedViews :many
SELECT v.id, v.org_id, v.project_id, v.created_by, v.name, v.layout, v.filters, v.created_at, v.updated_at
FROM views v
JOIN view_pins vp ON vp.view_id = v.id
WHERE vp.user_id = ?
ORDER BY vp.created_at DESC;

-- name: PinView :exec
INSERT OR IGNORE INTO view_pins (view_id, user_id) VALUES (?, ?);

-- name: UnpinView :exec
DELETE FROM view_pins WHERE view_id = ? AND user_id = ?;
