-- name: GetTrackMusicians :many
SELECT * FROM track_musicians
WHERE track_id = $1;

-- name: GetMusicianTracks :many
SELECT * FROM track_musicians
WHERE musician_id = $1;

-- name: CreateTrackMusician :exec
INSERT INTO track_musicians (
    track_id, musician_id
) VALUES (
    $1, $2
);

-- name: DeleteTrackMusician :exec
DELETE FROM track_musicians
WHERE track_id = $1 AND musician_id = $2; 