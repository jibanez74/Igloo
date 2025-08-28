-- name: CheckGenreExistByTag :one
SELECT EXISTS(
    SELECT 1 FROM genres WHERE tag = $1
) as exists;

-- name: GetGenreByTag :one
SELECT id, created_at, updated_at, tag, genre_type
FROM genres
WHERE tag = $1;

-- name: CreateGenre :one
INSERT INTO genres (
  tag,
  genre_type
) VALUES (
  $1, $2
)
RETURNING *;
