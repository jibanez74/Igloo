-- name: CreateMusician :one
INSERT INTO musicians (
    name,
    spotify_id,
    spotify_popularity,
    spotify_followers,
    summary,
    thumb
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: CheckMusicianExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM musicians WHERE spotify_id = $1
) as exists;

-- name: GetMusicianBySpotifyID :one
SELECT * FROM musicians WHERE spotify_id = $1;
