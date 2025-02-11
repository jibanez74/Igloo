-- name: CreateSubtitle :one
INSERT INTO subtitles (
    title,
    index,
    codec,
    language,
    movie_id
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;
