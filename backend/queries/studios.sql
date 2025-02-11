--- name: GetStudioByID
SELECT id, name, logo, tmdb_id FROM studios
WHERE id = $1;

-- name: GetStudioByTmdbID :one
SELECT id, tmdb_id FROM studios
WHERE tmdb_id = $1;

-- name: CreateStudio :one
INSERT INTO studios (
    name,
    country,
    logo,
    tmdb_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;
