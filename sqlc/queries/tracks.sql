-- name: CreateTrack :one
INSERT INTO tracks (
    title, index, duration, composer, release_date, file_path, container, codec, bit_rate, channel_layout, sample_rate, copyright, album_id, size, file_name, disc, album_id, language, profile
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
) RETURNING *;
