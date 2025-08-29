-- name: CheckAlbumGenreExist :one
SELECT EXISTS(
    SELECT 1 FROM album_genres WHERE album_id = $1 AND genre_id = $2
) as exists;

-- name: CreateAlbumGenre :exec
INSERT INTO album_genres (
    album_id, genre_id
) VALUES (
    $1, $2
);
