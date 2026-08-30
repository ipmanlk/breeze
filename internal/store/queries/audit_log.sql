-- name: CreateAuditEntry :exec
INSERT INTO audit_log (id, org_id, actor_id, action, entity_type, entity_id, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListAuditEntries :many
SELECT a.id, a.org_id, a.actor_id, a.action, a.entity_type, a.entity_id,
       a.metadata, a.created_at,
       u.name AS actor_name, u.email AS actor_email
FROM audit_log a
JOIN users u ON u.id = a.actor_id
WHERE a.org_id = @org_id
  AND (@action_filter IS NULL OR a.action = @action_filter)
  AND (@actor_id_filter IS NULL OR a.actor_id = @actor_id_filter)
ORDER BY a.created_at DESC
LIMIT @limit_val
OFFSET @offset_val;

-- name: CountAuditEntries :one
SELECT COUNT(*) FROM audit_log
WHERE org_id = @org_id
  AND (@action_filter IS NULL OR action = @action_filter)
  AND (@actor_id_filter IS NULL OR actor_id = @actor_id_filter);

-- name: DeleteAuditEntriesOlderThan :execrows
-- Audit log retention. Deletes entries older than the cutoff timestamp
-- across all orgs. Called by the periodic retention cleanup goroutine.
DELETE FROM audit_log WHERE created_at < sqlc.arg('cutoff');
