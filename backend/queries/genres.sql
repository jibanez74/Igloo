-- name: GetGenreByID :one
SELECt tag FROM genres
WHERE id = $1;

-- name: GetGenreByTmdbID :one
SELECT id, tag, tmdb_id FROM genres
WHERE tmdb_id = $1;

-- name: CreateGenre :one
INSERT INTO genres (
    tag,
    genre_type,
    tmdb_id
) VALUES (
    $1, $2, $3
)
RETURNING *;
