-- name: AddMovieStudio :exec
INSERT INTO movie_studios (movie_id, studio_id)
VALUES ($1, $2);

-- name: GetMovieStudios :many
SELECT s.* FROM studios s
JOIN movie_studios ms ON ms.studio_id = s.id
WHERE ms.movie_id = $1; 