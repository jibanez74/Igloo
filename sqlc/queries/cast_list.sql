-- name: CreateCastMember :one
INSERT INTO cast_list (
    artist_id,
    movie_id,
    character,
    sort_order
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;
