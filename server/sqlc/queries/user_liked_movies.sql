-- name: LikeMovie :exec
-- Idempotent: duplicate (user_id, movie_id) is a no-op (no error).
INSERT INTO user_liked_movies (user_id, movie_id)
VALUES (?, ?)
ON CONFLICT (user_id, movie_id) DO NOTHING;

-- name: UnlikeMovie :exec
-- Idempotent: deleting a non-existent row affects 0 rows and is not an error in SQLite.
DELETE FROM user_liked_movies WHERE user_id = ? AND movie_id = ?;

-- name: IsMovieLiked :one
SELECT COUNT(*) > 0 AS is_liked FROM user_liked_movies WHERE user_id = ? AND movie_id = ?;

-- name: CountUserLikedMovies :one
SELECT COUNT(*) FROM user_liked_movies WHERE user_id = ?;

-- name: GetLikedMoviesForUserAsc :many
-- One-line SELECT required for paginated rows (sqlc; see movies.sql).
SELECT m.id, m.title, m.poster_path, m.year, m.certification FROM user_liked_movies ulm INNER JOIN movies m ON m.id = ulm.movie_id WHERE ulm.user_id = ? ORDER BY LOWER(m.title) ASC LIMIT ? OFFSET ?;

-- name: GetLikedMoviesForUserDesc :many
SELECT m.id, m.title, m.poster_path, m.year, m.certification FROM user_liked_movies ulm INNER JOIN movies m ON m.id = ulm.movie_id WHERE ulm.user_id = ? ORDER BY LOWER(m.title) DESC LIMIT ? OFFSET ?;
