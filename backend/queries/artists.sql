--- name: GetArtistByID
SELECT id, name, original_name, thumb, tmdb_id FROM artists
WHERE id = $1;

-- name: GetOrCreateArtist :one
WITH existing_artist AS (
    SELECT a.id, a.created_at, a.updated_at, a.name, a.original_name, a.thumb, a.tmdb_id 
    FROM artists a
    WHERE a.tmdb_id = $1
    LIMIT 1
), new_artist AS (
    INSERT INTO artists (
        name,
        original_name,
        thumb,
        tmdb_id
    )
    SELECT $2, $3, $4, $1
    WHERE NOT EXISTS (SELECT 1 FROM existing_artist)
    RETURNING id, created_at, updated_at, name, original_name, thumb, tmdb_id
)
SELECT e.id, e.created_at, e.updated_at, e.name, e.original_name, e.thumb, e.tmdb_id 
FROM existing_artist e
UNION ALL
SELECT n.id, n.created_at, n.updated_at, n.name, n.original_name, n.thumb, n.tmdb_id 
FROM new_artist n;

