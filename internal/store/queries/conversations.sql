-- name: CreateConversation :one
INSERT INTO conversations (id, org_id, parent_id, name, topic, type, created_by, position_key)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetConversationByID :one
SELECT * FROM conversations WHERE id = ? AND org_id = ?;

-- name: GetConversationByIDAnyConv :one
SELECT * FROM conversations WHERE id = ?;

-- name: ListConversationsByUser :many
WITH RECURSIVE conv_with_link AS (
  -- anchor: conversations that directly have a project link
  SELECT c.id AS conv_id
  FROM conversations c
  WHERE EXISTS (SELECT 1 FROM channel_project_links cpl WHERE cpl.channel_id = c.id)
    AND c.org_id = @org_id AND c.deleted_at IS NULL
  UNION ALL
  -- recurse: children of linked conversations (descendants inherit access)
  SELECT c.id
  FROM conversations c
  JOIN conv_with_link p ON c.parent_id = p.conv_id
  WHERE c.org_id = @org_id AND c.deleted_at IS NULL
)
SELECT c.id, c.org_id, c.parent_id, c.name, c.topic, c.type, c.created_by, c.position_key, c.created_at, c.updated_at, c.deleted_at,
       (SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id) AS member_count
FROM conversations c
WHERE c.org_id = @org_id
  AND c.deleted_at IS NULL
  AND (
    -- explicit membership
    EXISTS (SELECT 1 FROM conversation_members cm WHERE cm.conversation_id = c.id AND cm.user_id = @user_id)
    OR (@include_project_linked = 1 AND c.id IN (SELECT conv_id FROM conv_with_link))
  )
  AND (
    @scope = ''
    OR (@scope = 'workspace' AND c.type IN ('channel', 'voice', 'category'))
    OR (@scope = 'dms' AND c.type IN ('direct', 'group'))
    OR (substr(@scope, 1, 8) = 'parent:' AND c.parent_id = substr(@scope, 9))
  )
  AND (
    @cursor_key = ''
    OR c.position_key > @cursor_key
    OR (c.position_key = @cursor_key AND c.id > @cursor_id)
  )
ORDER BY c.position_key ASC, c.id ASC
LIMIT @limit_val;

-- name: ListConversationsByParent :many
WITH RECURSIVE conv_with_link AS (
  -- anchor: conversations that directly have a project link
  SELECT c.id AS conv_id
  FROM conversations c
  WHERE EXISTS (SELECT 1 FROM channel_project_links cpl WHERE cpl.channel_id = c.id)
    AND c.org_id = @org_id AND c.deleted_at IS NULL
  UNION ALL
  -- recurse: children of linked conversations (descendants inherit access)
  SELECT c.id
  FROM conversations c
  JOIN conv_with_link p ON c.parent_id = p.conv_id
  WHERE c.org_id = @org_id AND c.deleted_at IS NULL
)
SELECT c.id, c.org_id, c.parent_id, c.name, c.topic, c.type, c.created_by, c.position_key, c.created_at, c.updated_at, c.deleted_at,
       (SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id) AS member_count
FROM conversations c
WHERE c.org_id = @org_id
  AND c.parent_id = @parent_id
  AND c.type IN ('channel', 'voice')
  AND c.deleted_at IS NULL
  AND (
    -- explicit membership
    EXISTS (SELECT 1 FROM conversation_members cm WHERE cm.conversation_id = c.id AND cm.user_id = @user_id)
    OR (@include_project_linked = 1 AND c.id IN (SELECT conv_id FROM conv_with_link))
  )
ORDER BY c.position_key ASC, c.id ASC;

-- name: ListCategoriesByOrg :many
SELECT * FROM conversations
WHERE org_id = @org_id
  AND type = 'category'
  AND deleted_at IS NULL
ORDER BY position_key ASC, id ASC;

-- name: ListChildPositionKeys :many
SELECT position_key FROM conversations
WHERE org_id = @org_id
  AND parent_id = @parent_id
  AND type IN ('channel', 'voice')
  AND deleted_at IS NULL
ORDER BY position_key ASC;

-- name: FindDMByUsers :one
SELECT c.id, c.org_id, c.parent_id, c.name, c.topic, c.type, c.created_by, c.position_key, c.created_at, c.updated_at, c.deleted_at FROM conversations c
INNER JOIN conversation_members requester ON requester.conversation_id = c.id AND requester.user_id = @requester_id
INNER JOIN conversation_members recipient ON recipient.conversation_id = c.id AND recipient.user_id = @recipient_id
WHERE c.org_id = @org_id AND c.type = 'direct' AND c.deleted_at IS NULL
LIMIT 1;

-- name: UpdateConversation :exec
UPDATE conversations SET name = ?, topic = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: UpdateConversationParent :exec
UPDATE conversations SET parent_id = ?, position_key = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: UpdateConversationPositionKey :exec
UPDATE conversations SET position_key = ?, updated_at = datetime('now')
WHERE id = ? AND org_id = ?;

-- name: DeleteConversation :exec
UPDATE conversations SET deleted_at = datetime('now') WHERE id = ? AND org_id = ?;

-- name: DeleteConversationsByParent :exec
UPDATE conversations SET deleted_at = datetime('now')
WHERE parent_id = ? AND org_id = ? AND deleted_at IS NULL;

-- name: AddConversationMember :exec
INSERT INTO conversation_members (conversation_id, user_id, org_id)
VALUES (?, ?, ?)
ON CONFLICT(conversation_id, user_id) DO NOTHING;

-- name: RemoveConversationMember :exec
DELETE FROM conversation_members WHERE conversation_id = ? AND user_id = ?;

-- name: ListConversationMembers :many
SELECT cm.*, u.name AS user_name, u.email AS user_email, u.avatar_url AS user_avatar_url, u.role AS user_role
FROM conversation_members cm
INNER JOIN users u ON u.id = cm.user_id
INNER JOIN conversations c ON c.id = cm.conversation_id AND c.org_id = cm.org_id
WHERE cm.conversation_id = ?
ORDER BY u.name ASC;

-- name: GetConversationMember :one
SELECT conversation_id, user_id, org_id, joined_at, last_read_at, muted FROM conversation_members WHERE conversation_id = ? AND user_id = ?;

-- name: CountConversationMembers :one
SELECT COUNT(*) FROM conversation_members WHERE conversation_id = ?;

-- name: UpdateLastRead :exec
UPDATE conversation_members SET last_read_at = datetime('now')
WHERE conversation_id = ? AND user_id = ?;

-- name: GetUnreadMessageCount :one
SELECT COUNT(*) FROM messages m
INNER JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = @user_id
WHERE m.conversation_id = @conversation_id
  AND m.deleted_at IS NULL
  AND m.created_at > cm.last_read_at
  AND m.sender_id != @user_id;

-- name: GetUnreadCounts :many
SELECT m.conversation_id, COUNT(*) AS unread
FROM messages m
INNER JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = @user_id
WHERE m.conversation_id IN (sqlc.slice('conversation_ids'))
  AND m.deleted_at IS NULL
  AND m.created_at > cm.last_read_at
  AND m.sender_id != @user_id
GROUP BY m.conversation_id;

-- name: GetLastMessage :one
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content,
       m.parent_id, m.forwarded_message_id,
       m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at
FROM messages m
WHERE m.conversation_id = ? AND m.deleted_at IS NULL
ORDER BY m.created_at DESC, m.id DESC
LIMIT 1;

-- name: CountPinnedMessages :one
SELECT COUNT(*) FROM messages WHERE conversation_id = ? AND pinned = 1 AND deleted_at IS NULL;

-- name: ListPinnedMessages :many
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content,
       m.parent_id, m.forwarded_message_id,
       m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at,
       u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.conversation_id = ? AND m.pinned = 1 AND m.deleted_at IS NULL
ORDER BY m.pinned_at DESC;

-- Channel project links
-- name: CreateChannelProjectLink :exec
INSERT INTO channel_project_links (channel_id, project_id) VALUES (?, ?)
ON CONFLICT(channel_id, project_id) DO NOTHING;

-- name: DeleteChannelProjectLink :exec
DELETE FROM channel_project_links WHERE channel_id = ? AND project_id = ?;

-- name: DeleteChannelProjectLinks :exec
DELETE FROM channel_project_links WHERE channel_id = ?;

-- name: GetChannelProjectLinks :many
SELECT project_id FROM channel_project_links WHERE channel_id = ?;

-- name: GetProjectChannelLinks :many
SELECT channel_id FROM channel_project_links WHERE project_id = ?;

-- name: UserHasProjectAccess :one
SELECT EXISTS (
    SELECT 1 FROM projects p
    WHERE p.id = @project_id AND p.org_id = @org_id
    AND (
        @user_role IN ('owner', 'admin', 'member')
        OR EXISTS (
            SELECT 1 FROM project_members pm
            WHERE pm.project_id = p.id AND pm.user_id = @user_id
        )
    )
) AS has_access;

-- name: GetUsersWithProjectAccess :many
SELECT DISTINCT u.id, u.name, u.email, u.avatar_url, u.role
FROM users u
WHERE u.org_id = @org_id AND (
    u.role IN ('owner', 'admin', 'member')
    OR EXISTS (
        SELECT 1 FROM project_members pm
        WHERE pm.project_id = @project_id AND pm.user_id = u.id
    )
)
ORDER BY u.name;

-- Channel permissions
-- name: GetChannelPermissions :many
SELECT role, permission, allow FROM channel_permissions WHERE channel_id = ?;

-- name: DeleteChannelPermissions :exec
DELETE FROM channel_permissions WHERE channel_id = ?;

-- name: CreateChannelPermission :exec
INSERT INTO channel_permissions (channel_id, role, permission, allow)
VALUES (?, ?, ?, ?);

-- Channel user overrides
-- name: GetChannelUserOverrides :many
SELECT user_id, permission, allow FROM channel_user_overrides WHERE channel_id = ?;

-- name: DeleteChannelUserOverrides :exec
DELETE FROM channel_user_overrides WHERE channel_id = ?;

-- name: CreateChannelUserOverride :exec
INSERT INTO channel_user_overrides (channel_id, user_id, permission, allow)
VALUES (?, ?, ?, ?)
ON CONFLICT(channel_id, user_id, permission) DO UPDATE SET allow = excluded.allow;

-- name: ListConversationsByIDs :many
SELECT * FROM conversations
WHERE org_id = @org_id AND id IN (sqlc.slice('ids'));
