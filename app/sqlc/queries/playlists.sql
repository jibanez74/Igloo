-- name: CreatePlaylist :one
INSERT INTO playlists (user_id, name, description, cover_image, is_public)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetPlaylistById :one
SELECT * FROM playlists WHERE id = ?;

-- name: GetPlaylistsByUserId :many
SELECT 
    p.*,
    (SELECT COUNT(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id) as track_count,
    (SELECT COALESCE(SUM(t.duration), 0) FROM playlist_tracks pt JOIN tracks t ON pt.track_id = t.id WHERE pt.playlist_id = p.id) as total_duration
FROM playlists p
WHERE p.user_id = ? 
ORDER BY p.updated_at DESC;

-- name: GetPlaylistsWithCollaboratorAccess :many
SELECT 
    p.*,
    (SELECT COUNT(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id) as track_count,
    (SELECT COALESCE(SUM(t.duration), 0) FROM playlist_tracks pt JOIN tracks t ON pt.track_id = t.id WHERE pt.playlist_id = p.id) as total_duration
FROM playlists p
LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
WHERE p.user_id = ? OR pc.user_id = ?
GROUP BY p.id
ORDER BY p.updated_at DESC;

-- name: CountPlaylistsByUserId :one
SELECT COUNT(*) as count FROM playlists WHERE user_id = ?;

-- name: UpdatePlaylist :one
UPDATE playlists 
SET name = ?, description = ?, cover_image = ?, is_public = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeletePlaylist :exec
DELETE FROM playlists WHERE id = ? AND user_id = ?;

-- name: UpdatePlaylistTimestamp :exec
UPDATE playlists SET updated_at = CURRENT_TIMESTAMP WHERE id = ?;
