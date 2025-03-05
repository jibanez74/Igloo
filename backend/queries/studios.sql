--- name: GetStudioByID
SELECT id, name, logo, tmdb_id FROM studios
WHERE id = $1;

-- name: GetOrCreateStudio :one
WITH existing_studio AS (
    SELECT s.id, s.created_at, s.updated_at, s.name, s.country, s.logo, s.tmdb_id 
    FROM studios s
    WHERE s.tmdb_id = $1
    LIMIT 1
), new_studio AS (
    INSERT INTO studios (
        name,
        country,
        logo,
        tmdb_id
    )
    SELECT $2, $3, $4, $1
    WHERE NOT EXISTS (SELECT 1 FROM existing_studio)
    RETURNING id, name, country, logo, tmdb_id
)
SELECT e.id, e.created_at, e.updated_at, e.name, e.country, e.logo, e.tmdb_id 
FROM existing_studio e
UNION ALL
SELECT n.id, n.created_at, n.updated_at, n.name, n.country, n.logo, n.tmdb_id 
FROM new_studio n;
