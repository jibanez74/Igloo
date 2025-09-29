-- name: CheckTrackExistByFilePath :one
SELECT EXISTS(
    SELECT 1 FROM tracks WHERE file_path = $1
) as exists;

-- name: CreateTrack :one
INSERT INTO tracks (
    title, sort_title, track_index, duration, composer, release_date, file_path, container, codec, bit_rate, channel_layout, copyright, size, file_name, disc, album_id, language, profile, sample_rate, musician_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
) RETURNING *;

-- name: GetTrackCount :one
SELECT COUNT(*) FROM tracks;

-- name: GetTrackByID :one
SELECT * FROM tracks WHERE id = $1;
