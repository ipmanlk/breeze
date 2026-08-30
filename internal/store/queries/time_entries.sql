-- name: ListTimeEntries :many
SELECT id, task_id, user_id, description, started_at, ended_at, duration_minutes, created_at, updated_at
FROM time_entries WHERE task_id = ? ORDER BY started_at DESC;

-- name: GetActiveTimer :one
SELECT id, task_id, user_id, description, started_at, ended_at, duration_minutes, created_at, updated_at
FROM time_entries WHERE task_id = ? AND user_id = ? AND ended_at IS NULL LIMIT 1;

-- name: GetActiveTimerByUser :many
SELECT id, task_id, user_id, description, started_at, ended_at, duration_minutes, created_at, updated_at
FROM time_entries WHERE user_id = ? AND ended_at IS NULL;

-- name: StartTimer :exec
INSERT INTO time_entries (id, task_id, user_id, description, started_at)
VALUES (?, ?, ?, ?, datetime('now'));

-- name: StopTimer :exec
UPDATE time_entries
SET ended_at = datetime('now'),
    duration_minutes = CAST(ROUND((julianday('now') - julianday(started_at)) * 24 * 60) AS INTEGER),
    updated_at = datetime('now')
WHERE id = ? AND user_id = ? AND ended_at IS NULL;

-- name: StopActiveTimersForUser :exec
UPDATE time_entries
SET ended_at = datetime('now'),
    duration_minutes = CAST(ROUND((julianday('now') - julianday(started_at)) * 24 * 60) AS INTEGER),
    updated_at = datetime('now')
WHERE user_id = ? AND ended_at IS NULL;

-- name: CreateTimeEntry :exec
INSERT INTO time_entries (id, task_id, user_id, description, started_at, ended_at, duration_minutes)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: UpdateTimeEntry :exec
UPDATE time_entries
SET description = ?, started_at = ?, ended_at = ?, duration_minutes = ?, updated_at = datetime('now')
WHERE id = ? AND task_id = ?;

-- name: DeleteTimeEntry :exec
DELETE FROM time_entries WHERE id = ? AND task_id = ?;

-- name: TotalTimeByTask :one
SELECT COALESCE(SUM(duration_minutes), 0) FROM time_entries WHERE task_id = ? AND duration_minutes IS NOT NULL;
