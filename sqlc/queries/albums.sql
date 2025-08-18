-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: GetAlbumsCount :one
SELECT COUNT(*) FROM albums;

-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    release_date,
    spotify_id,
    spotify_popularity,
    total_tracks,
    total_available_tracks,
    dir_path
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetAllAlbumsWithImages :many
SELECT 
    a.id, 
    a.title, 
    a.release_date,
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
