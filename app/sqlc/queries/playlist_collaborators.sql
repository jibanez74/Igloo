-- name: AddCollaborator :one
INSERT INTO playlist_collaborators (playlist_id, user_id, can_edit)
VALUES (?, ?, ?)
RETURNING *;

-- name: RemoveCollaborator :exec
DELETE FROM playlist_collaborators WHERE playlist_id = ? AND user_id = ?;

-- name: GetPlaylistCollaborators :many
SELECT 
    pc.*,
    u.name as username,
    u.email
FROM playlist_collaborators pc
JOIN users u ON pc.user_id = u.id
WHERE pc.playlist_id = ?
ORDER BY pc.created_at ASC;

-- name: IsUserCollaborator :one
SELECT EXISTS(
    SELECT 1 FROM playlist_collaborators 
    WHERE playlist_id = ? AND user_id = ?
) as is_collaborator;

-- name: CanUserEditPlaylist :one
SELECT EXISTS(
    SELECT 1 FROM playlists p
    LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
    WHERE p.id = ? AND (p.user_id = ? OR (pc.user_id = ? AND pc.can_edit = true))
) as can_edit;

-- name: UpdateCollaboratorPermission :exec
UPDATE playlist_collaborators SET can_edit = ? WHERE playlist_id = ? AND user_id = ?;
