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

-- name: GetSpotifyImageByPath :one
SELECT id, created_at, updated_at, path, width, height, musician_id, album_id
FROM spotify_images
WHERE path = $1;

-- name: CheckSpotifyImageExists :one
SELECT EXISTS(
    SELECT 1 FROM spotify_images WHERE path = $1
);

-- name: UpdateSpotifyImageRelations :exec
UPDATE spotify_images 
SET 
    musician_id = $2,
    album_id = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE path = $1;
