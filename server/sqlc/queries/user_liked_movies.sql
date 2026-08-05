-- name: LikeMovie :exec
-- Idempotent: duplicate (user_id, movie_id) is a no-op (no error).
INSERT INTO user_liked_movies (
  user_id,
  movie_id
)
VALUES
  (?, ?)
ON CONFLICT (user_id, movie_id) DO NOTHING;

-- name: IsMovieLiked :one
SELECT
  COUNT(*) > 0 AS is_liked
FROM user_liked_movies
WHERE user_id = ?
  AND movie_id = ?;

-- name: CountUserLikedMovies :one
SELECT
  COUNT(*)
FROM user_liked_movies
WHERE user_id = ?;

-- name: GetLikedMoviesForUserAsc :many
-- id tie-breaker so LIMIT/OFFSET is stable when titles match.
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  m.certification
FROM user_liked_movies AS ulm
INNER JOIN movies AS m
  ON m.id = ulm.movie_id
WHERE ulm.user_id = ?
ORDER BY
  LOWER(m.title) ASC,
  m.id ASC
LIMIT ?
OFFSET ?;

-- name: GetLikedMoviesForUserDesc :many
-- id tie-breaker so LIMIT/OFFSET is stable when titles match.
SELECT
  m.id,
  m.title,
  m.poster_path,
  m.year,
  m.certification
FROM user_liked_movies AS ulm
INNER JOIN movies AS m
  ON m.id = ulm.movie_id
WHERE ulm.user_id = ?
ORDER BY
  LOWER(m.title) DESC,
  m.id DESC
LIMIT ?
OFFSET ?;
