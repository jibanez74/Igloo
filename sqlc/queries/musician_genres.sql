-- name: CreateMusicianGenre :exec
INSERT INTO musician_genres (
    musician_id, genre_id
) VALUES (
    $1, $2
);
