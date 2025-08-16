-- name: GetGenreByID :one
SELECt tag FROM genres
WHERE id = $1;

-- name: GetOrCreateGenre :one
WITH existing_genre AS (
    SELECT g.id, g.created_at, g.updated_at, g.tag, g.genre_type
    FROM genres g
    WHERE g.tag = $1
    LIMIT 1
), new_genre AS (
    INSERT INTO genres (
        tag,
        genre_type
    )
    SELECT $1, $2
    WHERE NOT EXISTS (SELECT 1 FROM existing_genre)
    RETURNING id, created_at, updated_at, tag, genre_type
)
SELECT e.id, e.created_at, e.updated_at, e.tag, e.genre_type
FROM existing_genre e
UNION ALL
SELECT n.id, n.created_at, n.updated_at, n.tag, n.genre_type
FROM new_genre n;

-- name: GetAllGenres :many
SELECT id, tag, genre_type FROM genres;