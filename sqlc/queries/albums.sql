-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    sort_title,
    release_date,
    spotify_id,
    spotify_popularity,
    total_tracks,
    musician_id,
    cover,
    disc_count
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: CheckAlbumExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM albums WHERE spotify_id = $1
) as exists;
