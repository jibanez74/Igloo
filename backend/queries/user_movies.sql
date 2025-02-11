
-- name: GetUserMovies :many
SELECT m.* FROM movies m
JOIN user_movies um ON um.movie_id = m.id
WHERE um.user_id = $1; 

-- name: AddUserMovie :exec
INSERT INTO user_movies (user_id, movie_id)
VALUES ($1, $2);

-- name: RemoveUserMovie :exec
DELETE FROM user_movies
WHERE user_id = $1 AND movie_id = $2;
