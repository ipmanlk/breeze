-- name: AddInviteProject :exec
INSERT INTO invite_projects (invite_id, project_id, role)
VALUES (?, ?, ?);

-- name: ListInviteProjects :many
SELECT project_id, role
FROM invite_projects
WHERE invite_id = ?;

-- name: DeleteInviteProjects :exec
DELETE FROM invite_projects
WHERE invite_id = ?;
