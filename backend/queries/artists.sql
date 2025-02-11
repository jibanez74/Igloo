--- name: GetArtistByID
SELECT id, name, original_name, thumb, tmdb_id FROM artists
WHERE id = $1;

-- name: GetArtistByTmdbID :one
SELECT id, tmdb_id FROM artists
WHERE tmdb_id = $1
LIMIT 1;

-- name: CreateArtist :one
INSERT INTO artists (
    name,
    original_name,
    thumb,
    tmdb_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

