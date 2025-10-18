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
