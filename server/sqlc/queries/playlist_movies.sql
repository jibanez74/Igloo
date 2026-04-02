-- name: AddMovieToPlaylist :one
INSERT INTO playlist_movies (playlist_id, movie_id, position, added_by)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: RemoveMovieFromPlaylist :exec
DELETE FROM playlist_movies WHERE playlist_id = ? AND movie_id = ?;

-- name: CountPlaylistMovies :one
SELECT COUNT(*) FROM playlist_movies WHERE playlist_id = ?;

-- name: GetPlaylistMoviesPaginated :many
-- One-line SELECT required for paginated rows (sqlc; see movies.sql).
SELECT m.id, m.title, m.poster_path, m.year, m.certification FROM playlist_movies pm INNER JOIN movies m ON m.id = pm.movie_id WHERE pm.playlist_id = ? ORDER BY pm.position ASC, LOWER(m.title) ASC LIMIT ? OFFSET ?;
