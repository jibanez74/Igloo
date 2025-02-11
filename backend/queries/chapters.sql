-- name: CreateChapter :one
INSERT INTO chapters (
    title,
    start_time_ms,
    thumb,
    movie_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;
