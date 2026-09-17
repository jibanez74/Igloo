-- name: AddMovieToPlaylist :execrows
-- Zero rows affected means the movie was already in the playlist; the unique
-- constraint replaces a separate membership pre-check.
INSERT INTO playlist_movies (
  playlist_id,
  movie_id,
  position,
  added_by
)
VALUES
  (
    ?1,
    ?2,
    (
      SELECT COALESCE(MAX(position), -1) + 1
      FROM playlist_movies AS pm2
      WHERE pm2.playlist_id = ?1
    ),
    ?3
  )
ON CONFLICT (playlist_id, movie_id) DO NOTHING;

-- name: RemoveMovieFromPlaylist :exec
DELETE FROM playlist_movies
WHERE playlist_id = ?
  AND movie_id = ?;

-- name: CountPlaylistMovies :one
SELECT
  COUNT(*)
FROM playlist_movies
WHERE playlist_id = ?;

-- name: GetPlaylistMoviesPaginatedAsc :many
-- Title order matches GET /api/movies/library sort=asc.
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  m.certification
FROM playlist_movies AS pm
INNER JOIN movies AS m
  ON m.id = pm.movie_id
WHERE pm.playlist_id = ?
ORDER BY
  LOWER(m.title) ASC,
  m.id ASC
LIMIT ?
OFFSET ?;

-- name: GetPlaylistMoviesPaginatedDesc :many
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  m.certification
FROM playlist_movies AS pm
INNER JOIN movies AS m
  ON m.id = pm.movie_id
WHERE pm.playlist_id = ?
ORDER BY
  LOWER(m.title) DESC,
  m.id DESC
LIMIT ?
OFFSET ?;
