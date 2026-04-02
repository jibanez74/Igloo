-- name: CreatePlaylist :one
INSERT INTO playlists (user_id, name, description, cover_image, is_public, content_type)
VALUES (?, ?, ?, ?, ?, 'track')
RETURNING *;

-- name: GetPlaylistById :one
SELECT * FROM playlists WHERE id = ?;

-- name: GetPlaylistsWithCollaboratorAccess :many
SELECT
  p.*,
  (SELECT COUNT(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id) AS track_count,
  (SELECT COALESCE(SUM(t.duration), 0) FROM playlist_tracks pt
   JOIN tracks t ON pt.track_id = t.id WHERE pt.playlist_id = p.id) AS total_duration
FROM playlists p
LEFT JOIN playlist_collaborators pc ON p.id = pc.playlist_id
WHERE (p.user_id = ? OR pc.user_id = ?) AND p.content_type = 'track'
GROUP BY p.id
ORDER BY p.updated_at DESC;

-- name: UpdatePlaylist :one
UPDATE playlists
SET name = ?, description = ?, cover_image = ?, is_public = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeletePlaylist :exec
DELETE FROM playlists WHERE id = ? AND user_id = ?;

-- name: UpdatePlaylistTimestamp :exec
UPDATE playlists SET updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: GetMoviePlaylistsForUser :many
SELECT
  p.*,
  (SELECT COUNT(*) FROM playlist_movies pm WHERE pm.playlist_id = p.id) AS movie_count
FROM playlists p
WHERE p.user_id = ? AND p.content_type = 'movie'
ORDER BY p.updated_at DESC;
