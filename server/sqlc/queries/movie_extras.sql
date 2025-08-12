-- name: CreateMovieExtra :one
INSERT INTO movie_extras (
    title,
    url,
    kind,
    movie_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;
