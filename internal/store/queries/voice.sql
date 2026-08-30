-- name: ListVoiceParticipants :many
SELECT id, conversation_id, org_id, user_id, muted, deafened, connection_id, joined_at
FROM voice_participants
WHERE conversation_id = ? AND org_id = ?
ORDER BY joined_at ASC;

-- name: ListVoiceParticipantsWithUser :many
SELECT vp.id, vp.conversation_id, vp.org_id, vp.user_id, vp.muted, vp.deafened, vp.connection_id, vp.joined_at,
       u.name AS user_name, u.avatar_url AS user_avatar_url
FROM voice_participants vp
JOIN users u ON u.id = vp.user_id AND u.org_id = vp.org_id
WHERE vp.conversation_id = ? AND vp.org_id = ?
ORDER BY vp.joined_at ASC;

-- name: GetVoiceParticipant :one
SELECT id, conversation_id, org_id, user_id, muted, deafened, connection_id, joined_at
FROM voice_participants
WHERE conversation_id = ? AND user_id = ? AND org_id = ?;

-- name: JoinVoiceChannel :exec
INSERT INTO voice_participants (id, conversation_id, org_id, user_id, muted, deafened, connection_id)
VALUES (?, ?, ?, ?, 0, 0, ?);

-- name: LeaveVoiceChannel :exec
DELETE FROM voice_participants
WHERE conversation_id = ? AND user_id = ? AND org_id = ?;

-- name: UpdateVoiceFlags :exec
UPDATE voice_participants
SET muted = ?, deafened = ?
WHERE conversation_id = ? AND user_id = ? AND org_id = ?;

-- name: UpdateVoiceConnection :exec
-- Reassign a participant's owning connection (used when a tab takes over an
-- existing voice session).
UPDATE voice_participants
SET connection_id = ?
WHERE conversation_id = ? AND user_id = ? AND org_id = ?;

-- name: CountVoiceParticipants :one
SELECT COUNT(*) FROM voice_participants
WHERE conversation_id = ? AND org_id = ?;

-- name: ListActiveVoiceForUser :many
SELECT id, conversation_id, org_id, user_id, muted, deafened, connection_id, joined_at
FROM voice_participants WHERE user_id = ? AND org_id = ?;

-- name: DeleteAllVoiceParticipants :execrows
-- Startup sweep: voice_participants rows are in-memory session state that
-- outlives the process on a crash. Any row at boot refers to a connection
-- that no longer exists.
DELETE FROM voice_participants;
