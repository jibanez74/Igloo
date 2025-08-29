-- name: CheckMovieGenreExists :one
SELECT EXISTS(
  SELECT 1 FROM movie_genres WHERE movie_id = $1 AND genre_id = $2
) as exists;

-- name: CreateMovieGenre :exec
INSERT INTO movie_genres (
  movie_id,
  genre_id
) VALUES (
  $1, $2
);