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
-- collaborator branch excludes playlists the user owns.
--
-- Track count and total duration are correlated subqueries, deliberately. The
-- grouped-pass form that lived here had no correlation to the requesting user,
-- so SQLite materialized it by scanning the whole tracks table and aggregating
-- every playlist in the database to annotate this user's handful of rows. Per
-- row these are index probes on idx_playlist_tracks_position (and the
-- (playlist_id, track_id) primary key), which is bounded by the page the
-- handler actually returns.
--
-- artwork is a JSON array of the album covers of the playlist's first 24
-- tracks, in playlist order, for the card's collage. The handler keeps the
-- first four distinct ones. The 24 are taken before the covers are looked at,
-- so a long playlist without covers is still a short walk of
-- idx_playlist_tracks_position.
SELECT
  accessible.*,
  (
    SELECT COUNT(*)
    FROM playlist_tracks AS pt
    WHERE pt.playlist_id = accessible.id
  ) AS track_count,
  (
    SELECT COALESCE(SUM(t.duration), 0)
    FROM playlist_tracks AS pt
    INNER JOIN tracks AS t
      ON t.id = pt.track_id
    WHERE pt.playlist_id = accessible.id
  ) AS total_duration,
  CAST((
    SELECT json_group_array(first_track.cover)
    FROM (
      SELECT a.cover
      FROM (
        SELECT
          pt.track_id,
          pt.position,
          pt.id
        FROM playlist_tracks AS pt
        WHERE pt.playlist_id = accessible.id
        ORDER BY
          pt.position ASC,
          pt.id ASC
        LIMIT 24
      ) AS head
      INNER JOIN tracks AS t
        ON t.id = head.track_id
      INNER JOIN albums AS a
        ON a.id = t.album_id
      WHERE a.cover <> ''
      ORDER BY
        head.position ASC,
        head.id ASC
    ) AS first_track
  ) AS TEXT) AS artwork
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
-- artwork holds the posters of the first 24 movies in the order they were
-- added; the handler keeps the first four distinct ones.
SELECT
  accessible.*,
  (
    SELECT COUNT(*)
    FROM playlist_movies AS pm
    WHERE pm.playlist_id = accessible.id
  ) AS movie_count,
  CAST((
    SELECT json_group_array(first_movie.poster_path)
    FROM (
      SELECT m.poster_path
      FROM (
        SELECT
          pm.movie_id,
          pm.position,
          pm.id
        FROM playlist_movies AS pm
        WHERE pm.playlist_id = accessible.id
        ORDER BY
          pm.position ASC,
          pm.id ASC
        LIMIT 24
      ) AS head
      INNER JOIN movies AS m
        ON m.id = head.movie_id
      WHERE m.poster_path <> ''
      ORDER BY
        head.position ASC,
        head.id ASC
    ) AS first_movie
  ) AS TEXT) AS artwork
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
ORDER BY accessible.updated_at DESC;
