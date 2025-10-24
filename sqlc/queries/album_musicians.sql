-- name: CreateAlbumMusician :one
INSERT INTO album_musicians (
    album_id,
    musician_id
) VALUES (
    $1, $2
)
RETURNING *;

-- name: CheckAlbumMusicianExists :one
SELECT EXISTS(
    SELECT 1 FROM album_musicians 
    WHERE album_id = $1 AND musician_id = $2
);


