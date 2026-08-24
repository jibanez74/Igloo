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

-- name: GetPlaylistWithAccess :one
-- One seek for every playlist authorization decision: the playlist row by
-- primary key plus this user's collaborator row (if any) by the
-- (playlist_id, user_id) unique index. collaborator_can_edit is NULL when the
-- user is not a collaborator; the handler derives owner/edit/view from that
-- and p.user_id/p.is_public.
SELECT
  p.*,
  pc.can_edit AS collaborator_can_edit
FROM playlists AS p
LEFT JOIN playlist_collaborators AS pc
  ON pc.playlist_id = p.id
  AND pc.user_id = sqlc.arg(user_id)
WHERE p.id = sqlc.arg(playlist_id);

-- name: GetPlaylistsWithCollaboratorAccess :many
-- Playlists the user owns or collaborates on, newest-updated first. The two
-- access paths are separate indexed lookups (idx_playlist_user, then
-- idx_playlist_collaborators_user) glued with UNION ALL -- an OR would force a
-- scan of every user's playlists -- and they are disjoint because the
-- collaborator branch excludes playlists the user owns. Track count and total
-- duration come from one grouped pass over playlist_tracks instead of two
-- correlated subqueries per row.
SELECT
  accessible.*,
  COALESCE(agg.track_count, 0) AS track_count,
  COALESCE(agg.total_duration, 0) AS total_duration
FROM (
  SELECT
    p.*,
    CAST(1 AS BOOLEAN) AS is_owner,
    CAST(1 AS BOOLEAN) AS can_edit
  FROM playlists AS p
  WHERE p.user_id = sqlc.arg(requesting_user_id)
    AND p.content_type = 'track'
  UNION ALL
  SELECT
    p.*,
    CAST(0 AS BOOLEAN) AS is_owner,
    pc.can_edit
  FROM playlist_collaborators AS pc
  INNER JOIN playlists AS p
    ON p.id = pc.playlist_id
  WHERE pc.user_id = sqlc.arg(requesting_user_id)
    AND p.content_type = 'track'
    AND p.user_id <> sqlc.arg(requesting_user_id)
) AS accessible
LEFT JOIN (
  SELECT
    pt.playlist_id,
    COUNT(*) AS track_count,
    COALESCE(SUM(t.duration), 0) AS total_duration
  FROM playlist_tracks AS pt
  INNER JOIN tracks AS t
    ON pt.track_id = t.id
  GROUP BY pt.playlist_id
) AS agg
  ON agg.playlist_id = accessible.id
ORDER BY accessible.updated_at DESC;

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
-- The movie twin of GetPlaylistsWithCollaboratorAccess; see the notes there.
SELECT
  accessible.*,
  COALESCE(agg.movie_count, 0) AS movie_count
FROM (
  SELECT
    p.*,
    CAST(1 AS BOOLEAN) AS is_owner,
    CAST(1 AS BOOLEAN) AS can_edit
  FROM playlists AS p
  WHERE p.user_id = sqlc.arg(requesting_user_id)
    AND p.content_type = 'movie'
  UNION ALL
  SELECT
    p.*,
    CAST(0 AS BOOLEAN) AS is_owner,
    pc.can_edit
  FROM playlist_collaborators AS pc
  INNER JOIN playlists AS p
    ON p.id = pc.playlist_id
  WHERE pc.user_id = sqlc.arg(requesting_user_id)
    AND p.content_type = 'movie'
    AND p.user_id <> sqlc.arg(requesting_user_id)
) AS accessible
LEFT JOIN (
  SELECT
    pm.playlist_id,
    COUNT(*) AS movie_count
  FROM playlist_movies AS pm
  GROUP BY pm.playlist_id
) AS agg
  ON agg.playlist_id = accessible.id
ORDER BY accessible.updated_at DESC;
