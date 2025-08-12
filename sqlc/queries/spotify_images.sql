-- name: GetSpotifyImageByID :one
SELECT id, created_at, updated_at, path, width, height, musician_id, album_id 
FROM spotify_images 
WHERE id = $1;

-- name: GetSpotifyImagesByMusicianID :many
SELECT id, created_at, updated_at, path, width, height, musician_id, album_id 
FROM spotify_images 
WHERE musician_id = $1
ORDER BY created_at DESC;

-- name: GetSpotifyImagesByAlbumID :many
SELECT id, created_at, updated_at, path, width, height, musician_id, album_id 
FROM spotify_images 
WHERE album_id = $1
ORDER BY created_at DESC;

-- name: ListSpotifyImages :many
SELECT id, created_at, updated_at, path, width, height, musician_id, album_id 
FROM spotify_images 
ORDER BY created_at DESC;

-- name: CreateSpotifyImage :one
INSERT INTO spotify_images (
    path,
    width,
    height,
    musician_id,
    album_id
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING id, created_at, updated_at, path, width, height, musician_id, album_id;

-- name: UpdateSpotifyImage :one
UPDATE spotify_images 
SET 
    path = $2,
    width = $3,
    height = $4,
    musician_id = $5,
    album_id = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, created_at, updated_at, path, width, height, musician_id, album_id;

-- name: DeleteSpotifyImage :exec
DELETE FROM spotify_images 
WHERE id = $1;

-- name: DeleteSpotifyImagesByMusicianID :exec
DELETE FROM spotify_images 
WHERE musician_id = $1;

-- name: DeleteSpotifyImagesByAlbumID :exec
DELETE FROM spotify_images 
WHERE album_id = $1;

-- name: GetMusicianWithImages :many
SELECT 
    m.id as musician_id,
    m.created_at as musician_created_at,
    m.updated_at as musician_updated_at,
    m.name as musician_name,
    m.summary as musician_summary,
    m.spotify_id as musician_spotify_id,
    m.spotify_popularity as musician_spotify_popularity,
    m.spotify_followers as musician_spotify_followers,
    si.id as image_id,
    si.created_at as image_created_at,
    si.updated_at as image_updated_at,
    si.path as image_path,
    si.width as image_width,
    si.height as image_height
FROM musicians m
LEFT JOIN spotify_images si ON m.id = si.musician_id
WHERE m.id = $1
ORDER BY si.created_at DESC;

-- name: GetAlbumWithImages :many
SELECT 
    a.id as album_id,
    a.created_at as album_created_at,
    a.updated_at as album_updated_at,
    a.title as album_title,
    a.spotify_id as album_spotify_id,
    a.release_date as album_release_date,
    a.spotify_popularity as album_spotify_popularity,
    a.total_tracks as album_total_tracks,
    a.total_available_tracks as album_total_available_tracks,
    si.id as image_id,
    si.created_at as image_created_at,
    si.updated_at as image_updated_at,
    si.path as image_path,
    si.width as image_width,
    si.height as image_height
FROM albums a
LEFT JOIN spotify_images si ON a.id = si.album_id
WHERE a.id = $1
ORDER BY si.created_at DESC; 