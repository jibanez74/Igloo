-- name: GetTrackGenresByAlbumID :many
SELECT tg.track_id, tg.genre_id
FROM track_genres tg
JOIN tracks t ON t.id = tg.track_id
WHERE t.album_id = $1;

-- name: CheckTrackGenreExists :one
SELECT EXISTS(
    SELECT 1 FROM track_genres WHERE track_id = $1 AND genre_id = $2
) as exists;

-- name: CreateTrackGenre :exec
INSERT INTO track_genres (
    track_id, genre_id
) VALUES (
    $1, $2
);
