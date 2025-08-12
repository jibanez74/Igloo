-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;
-- name: GetAllAlbums :many
SELECT 
    a.id, a.title, a.release_date,
    si.id as image_id, si.path as image_path, si.width as image_width, si.height as image_height
FROM albums a
LEFT JOIN spotify_images si ON a.id = si.album_id
ORDER BY a.title ASC;

-- name: GetAlbumsCount :one
SELECT COUNT(*) FROM albums;

-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    spotify_id,
    release_date,
    spotify_popularity,
    total_tracks,
    total_available_tracks
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: UpdateAlbum :one
UPDATE albums SET
    title = $1,
    spotify_id = $2,
    release_date = $3,
    spotify_popularity = $4,
    total_tracks = $5,
    total_available_tracks = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $7
RETURNING *;

-- name: GetAllAlbumsWithImages :many
SELECT 
    a.id, 
    a.title, 
    a.year,
    si.id as image_id, 
    si.path as image_path, 
    si.width as image_width, 
    si.height as image_height
FROM albums a
LEFT JOIN spotify_images si ON a.id = si.album_id
ORDER BY a.title ASC;

-- name: CheckAlbumExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM albums WHERE spotify_id = $1
) as exists;
