-- name: CreateCrewMember :one
INSERT INTO crew_list (
    artist_id,
    movie_id,
    job,
    department
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;
