-- name: GetTrack :one
SELECT * FROM tracks
WHERE id = $1 LIMIT 1;

-- name: GetTrackByFilePath :one
SELECT * FROM tracks
WHERE file_path = $1 LIMIT 1;

-- name: ListTracks :many
SELECT * FROM tracks
ORDER BY title;

-- name: ListTracksByAlbum :many
SELECT * FROM tracks
WHERE album_id = $1
ORDER BY index;

-- name: CreateTrack :one
INSERT INTO tracks (
    title, index, duration, composer, release_date, file_path, container, codec, bit_rate, channels, sample_rate, bit_depth, album_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: UpdateTrack :one
UPDATE tracks
SET title = $2, index = $3, duration = $4, composer = $5, release_date = $6, file_path = $7, container = $8, codec = $9, bit_rate = $10, channels = $11, sample_rate = $12, bit_depth = $13, album_id = $14, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteTrack :exec
DELETE FROM tracks
WHERE id = $1; 