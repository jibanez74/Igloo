-- name: GetGenreByTagAndType :one
SELECT * FROM genres WHERE tag = $1 AND genre_type = $2;

-- name: CreateGenre :one
INSERT INTO genres (
  tag,
  genre_type
) VALUES (
  $1, $2
)
RETURNING *;
