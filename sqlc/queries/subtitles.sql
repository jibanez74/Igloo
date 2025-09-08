-- name: CreateSubtitle :one
INSERT INTO subtitles (
    title,
    subtitle_index,
    codec,
    language,
    movie_id
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;
