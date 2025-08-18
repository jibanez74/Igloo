-- name: UpsertSpotifyImage :one
INSERT INTO spotify_images (
    path,
    width,
    height,
    musician_id,
    album_id
) VALUES (
    $1, $2, $3, $4, $5
) ON CONFLICT (path) DO UPDATE SET
    musician_id = EXCLUDED.musician_id,
    album_id = EXCLUDED.album_id,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, created_at, updated_at, path, width, height, musician_id, album_id;
