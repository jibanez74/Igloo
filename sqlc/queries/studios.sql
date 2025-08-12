--- name: GetStudioByID :one
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
    RETURNING id, created_at, updated_at, name, country, logo, tmdb_id
)
SELECT id, created_at, updated_at, name, country, logo, tmdb_id 
FROM existing_studio
UNION ALL
SELECT id, created_at, updated_at, name, country, logo, tmdb_id 
FROM new_studio;
