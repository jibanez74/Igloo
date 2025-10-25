-- name: GetAlbumCount :one
SELECT COUNT(*) FROM albums;

-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: GetAlbumByTitle :one
SELECT * FROM albums Where title = $1;

-- name: GetLatestAlbums :many
SELECT 
    id,
    title,
    musician,
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
    musician,
    cover
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

