-- name: CreateMessage :one
INSERT INTO messages (id, conversation_id, org_id, sender_id, content, search_content, parent_id, forwarded_message_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateAttachmentMessageID :exec
UPDATE message_attachments SET message_id = ? WHERE id = ?;

-- name: GetMessageByID :one
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content, m.parent_id, m.forwarded_message_id, m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at, u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.id = ? AND m.conversation_id = ? AND m.deleted_at IS NULL;

-- name: GetMessageByIDAnyConv :one
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content, m.parent_id, m.forwarded_message_id, m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at, u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.id = ? AND m.deleted_at IS NULL;

-- name: ListMessages :many
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content, m.parent_id, m.forwarded_message_id, m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at, u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.conversation_id = @conversation_id AND m.org_id = @org_id AND m.deleted_at IS NULL
  AND (
    @cursor_created_at = '' OR @cursor_id = ''
    OR m.created_at < @cursor_created_at
    OR (m.created_at = @cursor_created_at AND m.id < @cursor_id)
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT @limit_val;

-- name: ListReplies :many
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content, m.parent_id, m.forwarded_message_id, m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at, u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.conversation_id = @conversation_id AND m.org_id = @org_id AND m.parent_id = @parent_id AND m.deleted_at IS NULL
  AND (
    @cursor_created_at = '' OR @cursor_id = ''
    OR m.created_at < @cursor_created_at
    OR (m.created_at = @cursor_created_at AND m.id < @cursor_id)
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT @limit_val;

-- name: UpdateMessageContent :exec
UPDATE messages
SET content = ?, search_content = ?, edited_at = datetime('now')
WHERE id = ? AND conversation_id = ? AND sender_id = ?;

-- name: SoftDeleteMessage :exec
UPDATE messages
SET deleted_at = datetime('now')
WHERE id = ? AND conversation_id = ?;

-- name: PinMessage :exec
UPDATE messages
SET pinned = 1, pinned_at = datetime('now'), pinned_by = ?
WHERE id = ? AND conversation_id = ?;

-- name: UnpinMessage :exec
UPDATE messages
SET pinned = 0, pinned_at = NULL, pinned_by = NULL
WHERE id = ? AND conversation_id = ?;

-- name: CreateMessageAttachment :exec
INSERT INTO message_attachments (id, message_id, file_name, file_size, content_type, storage_path)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListMessageAttachments :many
SELECT id, message_id, file_name, file_size, content_type, storage_path, created_at FROM message_attachments
WHERE message_id = ?
ORDER BY created_at ASC;

-- name: GetMessageAttachmentByID :one
SELECT id, message_id, file_name, file_size, content_type, storage_path, created_at FROM message_attachments WHERE id = ?;

-- name: GetMessageAttachmentByIDAndConversation :one
-- Fetch an attachment by ID, joining to its message to verify the attachment
-- actually belongs to the given conversation. Used by the download endpoint
-- to prevent cross-conversation IDOR (passing convA in the path while
-- requesting an attachment from convB).
SELECT ma.id, ma.message_id, ma.file_name, ma.file_size, ma.content_type, ma.storage_path, ma.created_at
FROM message_attachments ma
JOIN messages m ON m.id = ma.message_id
WHERE ma.id = ? AND m.conversation_id = ?;

-- name: DeleteMessageAttachment :exec
DELETE FROM message_attachments WHERE id = ?;

-- name: CountMessages :one
SELECT COUNT(*) FROM messages WHERE conversation_id = ? AND deleted_at IS NULL;

-- name: GetConversationLastMessage :one
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content, m.parent_id, m.forwarded_message_id, m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at, u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.conversation_id = ? AND m.deleted_at IS NULL
ORDER BY m.created_at DESC, m.id DESC
LIMIT 1;

-- name: GetLastMessagesForConversations :many
SELECT m.id, m.conversation_id, m.org_id, m.sender_id, m.content, m.search_content,
       m.parent_id, m.forwarded_message_id,
       m.pinned, m.pinned_at, m.pinned_by, m.edited_at, m.deleted_at, m.created_at,
       u.name AS sender_name, u.email AS sender_email, u.avatar_url AS sender_avatar_url
FROM messages m
INNER JOIN users u ON u.id = m.sender_id
WHERE m.id IN (
  SELECT m2.id FROM messages m2
  WHERE m2.deleted_at IS NULL
    AND m2.conversation_id IN (sqlc.slice('conversation_ids'))
    AND m2.id = (
      SELECT m3.id FROM messages m3
      WHERE m3.conversation_id = m2.conversation_id AND m3.deleted_at IS NULL
      ORDER BY m3.created_at DESC, m3.id DESC
      LIMIT 1
    )
)
ORDER BY m.created_at DESC;

-- name: AddReaction :exec
INSERT INTO message_reactions (message_id, user_id, org_id, emoji)
VALUES (?, ?, ?, ?)
ON CONFLICT(message_id, user_id, emoji) DO NOTHING;

-- name: RemoveReaction :execrows
DELETE FROM message_reactions WHERE message_id = ? AND user_id = ? AND emoji = ?;

-- name: ListReactions :many
SELECT message_id, user_id, org_id, emoji, created_at
FROM message_reactions
WHERE message_id IN (sqlc.slice('message_ids'))
ORDER BY created_at ASC;

-- name: GetReactionsForMessages :many
SELECT message_id, user_id, org_id, emoji, created_at
FROM message_reactions
WHERE message_id IN (sqlc.slice('message_ids'))
ORDER BY created_at ASC;

-- name: ListReactionCounts :many
SELECT message_id, emoji, COUNT(*) AS count
FROM message_reactions
WHERE message_id IN (sqlc.slice('message_ids'))
GROUP BY message_id, emoji
ORDER BY message_id, emoji;

-- name: SearchMessages :many
-- Use a CTE so MATCH lives in its own isolated scope; SQLite FTS5 forbids
-- MATCH in an OR context when joined to real tables. UNION covers both the
-- indexed `content` column and `attachment_names` column.
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
),
fts_results AS (
  SELECT
    message_id,
    bm25(messages_fts) AS rank_val,
    snippet(messages_fts, 3, '<mark>', '</mark>', '...', 48) AS snippet
  FROM messages_fts
  WHERE messages_fts.content MATCH sqlc.arg(query)
  UNION
  SELECT
    message_id,
    bm25(messages_fts) AS rank_val,
    snippet(messages_fts, 3, '<mark>', '</mark>', '...', 48) AS snippet
  FROM messages_fts
  WHERE messages_fts.attachment_names MATCH sqlc.arg(query)
)
SELECT
  m.id,
  m.conversation_id,
  m.org_id,
  m.sender_id,
  m.content,
  m.parent_id,
  m.forwarded_message_id,
  m.pinned,
  m.pinned_at,
  m.pinned_by,
  m.edited_at,
  m.deleted_at,
  m.created_at,
  u.name AS sender_name,
  u.email AS sender_email,
  u.avatar_url AS sender_avatar_url,
  c.name AS conversation_name,
  c.type AS conversation_type,
  CASE WHEN ma.message_id IS NOT NULL THEN 1 ELSE 0 END AS has_attachment,
  f.rank_val,
  f.snippet
FROM fts_results f
JOIN messages m ON m.id = f.message_id
JOIN users u ON u.id = m.sender_id
JOIN conversations c ON c.id = m.conversation_id
LEFT JOIN (
  SELECT message_id FROM message_attachments GROUP BY message_id
) ma ON ma.message_id = m.id
LEFT JOIN conversation_members cm
  ON cm.conversation_id = m.conversation_id AND cm.user_id = @user_id
WHERE m.org_id = @org_id
  AND m.deleted_at IS NULL
  AND (
    cm.user_id IS NOT NULL
    OR (@include_project_linked = 1 AND c.id IN (SELECT conv_id FROM conv_with_link))
  )
  AND (
    @scope = 'all'
    OR (@scope = 'workspace' AND c.type = 'channel')
    OR (@scope = 'dm' AND c.type IN ('direct', 'group'))
  )
  AND (@conversation_id = '' OR m.conversation_id = @conversation_id)
  AND (@sender_id = '' OR m.sender_id = @sender_id)
  AND (@has_attachment = 0 OR ma.message_id IS NOT NULL)
  AND (@has_link = 0 OR m.content LIKE '%http://%' OR m.content LIKE '%https://%')
  AND (@is_pinned = 0 OR m.pinned = 1)
  AND (@after = '' OR m.created_at >= @after)
  AND (@before = '' OR m.created_at <= @before)
  AND (
    @cursor_created_at = '' OR @cursor_id = ''
    OR m.created_at < @cursor_created_at
    OR (m.created_at = @cursor_created_at AND m.id < @cursor_id)
  )
ORDER BY f.rank_val ASC, m.created_at DESC, m.id DESC
LIMIT @limit_val;
