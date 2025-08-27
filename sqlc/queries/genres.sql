-- name: UpsertGenre :one
INSERT INTO genres (tag, genre_type)
VALUES ($1, $2)
ON CONFLICT (tag) DO UPDATE SET tag = genres.tag
RETURNING id, created_at, updated_at, tag, genre_type;

-- name: GetGenreByTag :one
SELECT id, created_at, updated_at, tag, genre_type
FROM genres
WHERE tag = $1;

-- name: GetUnknownGenre :one
SELECT id, created_at, updated_at, tag, genre_type
FROM genres
WHERE tag = 'unknown';
