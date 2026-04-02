-- name: AddMovieToPlaylist :one
INSERT INTO playlist_movies (playlist_id, movie_id, position, added_by)
VALUES (
  ?1,
  ?2,
  (SELECT COALESCE(MAX(position), -1) + 1 FROM playlist_movies pm2 WHERE pm2.playlist_id = ?1),
  ?3
)
RETURNING *;

-- name: IsMovieInPlaylist :one
SELECT EXISTS(SELECT 1 FROM playlist_movies WHERE playlist_id = ? AND movie_id = ?) AS is_in_playlist;

-- name: RemoveMovieFromPlaylist :exec
DELETE FROM playlist_movies WHERE playlist_id = ? AND movie_id = ?;

-- name: CountPlaylistMovies :one
SELECT COUNT(*) FROM playlist_movies WHERE playlist_id = ?;

-- name: GetPlaylistMoviesPaginatedAsc :many
-- One-line SELECT required for paginated rows (sqlc; see movies.sql). Title order matches GET /api/movies/library sort=asc.
SELECT m.id, m.title, m.poster_path, m.year, m.certification FROM playlist_movies pm INNER JOIN movies m ON m.id = pm.movie_id WHERE pm.playlist_id = ? ORDER BY LOWER(m.title) ASC, m.id ASC LIMIT ? OFFSET ?;

-- name: GetPlaylistMoviesPaginatedDesc :many
SELECT m.id, m.title, m.poster_path, m.year, m.certification FROM playlist_movies pm INNER JOIN movies m ON m.id = pm.movie_id WHERE pm.playlist_id = ? ORDER BY LOWER(m.title) DESC, m.id DESC LIMIT ? OFFSET ?;
