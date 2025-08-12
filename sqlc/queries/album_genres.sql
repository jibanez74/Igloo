-- name: GetAlbumGenres :many
SELECT * FROM album_genres
WHERE album_id = $1;

-- name: GetGenreAlbums :many
SELECT * FROM album_genres
WHERE genre_id = $1;

-- name: CreateAlbumGenre :exec
INSERT INTO album_genres (
    album_id, genre_id
) VALUES (
    $1, $2
);

-- name: DeleteAlbumGenre :exec
DELETE FROM album_genres
WHERE album_id = $1 AND genre_id = $2; 