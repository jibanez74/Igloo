-- name: AddMovieGenre :exec
INSERT INTO movie_genres (movie_id, genre_id)
VALUES ($1, $2);
