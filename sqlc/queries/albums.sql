-- name: GetAlbumCount :one
SELECT COUNT(*) FROM albums;

-- name: CheckAlbumExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM albums WHERE spotify_id = $1
) as exists;

-- name: GetAlbumByTitle :one
SELECT * FROM albums Where title = $1;

-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    sort_title,
    release_date,
    year,
    spotify_id,
    spotify_popularity,
    total_tracks,
    musician_id,
    cover,
    disc_count
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;
