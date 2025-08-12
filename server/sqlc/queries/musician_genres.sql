-- name: GetMusicianGenres :many
SELECT * FROM musician_genres
WHERE musician_id = $1;

-- name: GetGenreMusicians :many
SELECT * FROM musician_genres
WHERE genre_id = $1;

-- name: CreateMusicianGenre :exec
INSERT INTO musician_genres (
    musician_id, genre_id
) VALUES (
    $1, $2
);

-- name: DeleteMusicianGenre :exec
DELETE FROM musician_genres
WHERE musician_id = $1 AND genre_id = $2; 