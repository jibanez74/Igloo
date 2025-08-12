-- name: CreateAlbumMusician :exec
INSERT INTO album_musicians (
    album_id, musician_id
) VALUES (
    $1, $2
);

-- name: DeleteAlbumMusician :exec
DELETE FROM album_musicians
WHERE album_id = $1 AND musician_id = $2;

-- name: GetAlbumMusicians :many
SELECT album_id, musician_id FROM album_musicians
WHERE album_id = $1;

-- name: GetMusicianAlbums :many
SELECT album_id, musician_id FROM album_musicians
WHERE musician_id = $1; 