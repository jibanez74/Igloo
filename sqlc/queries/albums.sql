-- name: GetAlbumCount :one
SELECT COUNT(*) FROM albums;

-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: GetAlbumByTitle :one
SELECT * FROM albums Where title = $1;

-- name: GetAlbumsPaginated :many
SELECT 
    a.id,
    a.title,
    a.cover,
    m.name as musician_name
FROM albums a
LEFT JOIN musicians m ON m.id = a.musician_id
ORDER BY a.sort_title ASC
LIMIT $1 OFFSET $2;

-- name: GetLatestAlbums :many
SELECT 
    id,
    title,
    cover
FROM albums
ORDER BY created_at DESC
LIMIT 12;

-- name: GetAlbumDetails :one
SELECT * FROM albums WHERE id = $1;
-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    sort_title,
    spotify_id,
    release_date,
    year,
    spotify_popularity,
    total_tracks,
    cover
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

