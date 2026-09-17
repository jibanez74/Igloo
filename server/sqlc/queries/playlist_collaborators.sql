-- name: AddCollaborator :one
INSERT INTO playlist_collaborators (
  playlist_id,
  user_id,
  can_edit
)
VALUES
  (?, ?, ?)
RETURNING *;

-- name: RemoveCollaborator :exec
DELETE FROM playlist_collaborators
WHERE playlist_id = ?
  AND user_id = ?;

-- name: GetPlaylistCollaborators :many
SELECT
  pc.*,
  u.name AS username,
  u.email
FROM playlist_collaborators AS pc
INNER JOIN users AS u
  ON pc.user_id = u.id
WHERE pc.playlist_id = ?
ORDER BY pc.created_at ASC;

