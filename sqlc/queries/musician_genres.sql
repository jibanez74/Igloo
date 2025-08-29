-- name: CheckMusicianGenreExist :one
SELECT EXISTS(
    SELECT 1 FROM musician_genres WHERE musician_id = $1 AND genre_id = $2
) as exists;

-- name: CreateMusicianGenre :exec
INSERT INTO musician_genres (
    musician_id, genre_id
) VALUES (
    $1, $2
);
