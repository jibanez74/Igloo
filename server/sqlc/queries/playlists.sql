-- name: CreatePlaylist :one
INSERT INTO playlists (
  user_id,
  name,
  description,
  cover_image,
  is_public,
  content_type
)
VALUES
  (?, ?, ?, ?, ?, 'track')
RETURNING *;

-- name: GetPlaylistById :one
SELECT
  *
FROM playlists
WHERE id = ?;

-- name: GetPlaylistsWithCollaboratorAccess :many
SELECT
  p.*,
  (
    SELECT COUNT(*)
    FROM playlist_tracks AS pt
    WHERE pt.playlist_id = p.id
  ) AS track_count,
  (
    SELECT COALESCE(SUM(t.duration), 0)
    FROM playlist_tracks AS pt
    INNER JOIN tracks AS t
      ON pt.track_id = t.id
    WHERE pt.playlist_id = p.id
  ) AS total_duration,
  EXISTS (
    SELECT 1
    WHERE p.user_id = sqlc.arg(requesting_user_id)
  ) AS is_owner,
  EXISTS (
    SELECT 1
    WHERE p.user_id = sqlc.arg(requesting_user_id)
      OR EXISTS (
        SELECT 1
        FROM playlist_collaborators AS edit_pc
        WHERE edit_pc.playlist_id = p.id
          AND edit_pc.user_id = sqlc.arg(requesting_user_id)
          AND edit_pc.can_edit = true
      )
  ) AS can_edit
FROM playlists AS p
WHERE (
    p.user_id = sqlc.arg(requesting_user_id)
    OR EXISTS (
      SELECT 1
      FROM playlist_collaborators AS access_pc
      WHERE access_pc.playlist_id = p.id
        AND access_pc.user_id = sqlc.arg(requesting_user_id)
    )
  )
  AND p.content_type = 'track'
ORDER BY p.updated_at DESC;

-- name: UpdatePlaylist :one
UPDATE playlists
SET
  name = ?,
  description = ?,
  cover_image = ?,
  is_public = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeletePlaylist :exec
DELETE FROM playlists
WHERE id = ?
  AND user_id = ?;

-- name: UpdatePlaylistTimestamp :exec
UPDATE playlists
SET updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CreateMoviePlaylist :one
INSERT INTO playlists (
  user_id,
  name,
  description,
  cover_image,
  is_public,
  content_type,
  movie_id
)
VALUES
  (?, ?, ?, ?, ?, 'movie', ?)
RETURNING *;

-- name: UpdateMoviePlaylist :one
UPDATE playlists
SET
  name = ?,
  description = ?,
  cover_image = ?,
  is_public = ?,
  movie_id = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
  AND user_id = ?
  AND content_type = 'movie'
RETURNING *;

-- name: GetMoviePlaylistsWithCollaboratorAccess :many
SELECT
  p.id,
  p.user_id,
  p.name,
  p.description,
  p.cover_image,
  p.is_public,
  p.folder_id,
  p.movie_id,
  p.content_type,
  p.created_at,
  p.updated_at,
  (
    SELECT COUNT(*)
    FROM playlist_movies AS pm
    WHERE pm.playlist_id = p.id
  ) AS movie_count,
  EXISTS (
    SELECT 1
    WHERE p.user_id = sqlc.arg(requesting_user_id)
  ) AS is_owner,
  EXISTS (
    SELECT 1
    WHERE p.user_id = sqlc.arg(requesting_user_id)
      OR EXISTS (
        SELECT 1
        FROM playlist_collaborators AS edit_pc
        WHERE edit_pc.playlist_id = p.id
          AND edit_pc.user_id = sqlc.arg(requesting_user_id)
          AND edit_pc.can_edit = true
      )
  ) AS can_edit
FROM playlists AS p
WHERE (
    p.user_id = sqlc.arg(requesting_user_id)
    OR EXISTS (
      SELECT 1
      FROM playlist_collaborators AS access_pc
      WHERE access_pc.playlist_id = p.id
        AND access_pc.user_id = sqlc.arg(requesting_user_id)
    )
  )
  AND p.content_type = 'movie'
ORDER BY p.updated_at DESC;
