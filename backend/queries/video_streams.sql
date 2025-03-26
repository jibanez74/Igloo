-- name: CreateVideoStream :one
INSERT INTO video_streams (
    title,
    index,
    duration,
    profile,
    aspect_ratio,
    bit_rate,
    bit_depth,
    codec,
    width,
    height,
    coded_width,
    coded_height,
    color_transfer,
    color_primaries,
    color_space,
    color_range,
    frame_rate,
    avg_frame_rate,
    level,
    movie_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
)
RETURNING *;

-- name: GetMovieVideoStreams :many
SELECT * FROM video_streams
WHERE movie_id = $1
ORDER BY index;
