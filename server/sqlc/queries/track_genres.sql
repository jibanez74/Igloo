-- name: GetTrackGenres :many
SELECT * FROM track_genres
WHERE track_id = $1;

-- name: GetGenreTracks :many
SELECT * FROM track_genres
WHERE genre_id = $1;

-- name: CreateTrackGenre :exec
INSERT INTO track_genres (
    track_id, genre_id
) VALUES (
    $1, $2
);

-- name: DeleteTrackGenre :exec
DELETE FROM track_genres
WHERE track_id = $1 AND genre_id = $2;

