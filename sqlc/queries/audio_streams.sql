-- name: CreateAudioStream :one
INSERT INTO audio_streams (
    title,
    audio_index,
    codec,
    channels,
    channel_layout,
    language,
    movie_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;
