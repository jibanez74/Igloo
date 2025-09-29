-- name: GetMusicianCount :one
SELECT COUNT(*) FROM musicians;

-- name: CheckMusicianExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM musicians WHERE spotify_id = $1
) as exists;

-- name: GetMusicianBySpotifyID :one
SELECT * FROM musicians WHERE spotify_id = $1;

-- name: GetMusicianByName :one
SELECT * FROM musicians WHERE name = $1;

-- name: GetMusicianByID :one
SELECT * FROM musicians WHERE id = $1 ORDER BY sort_name;

-- name: GetMusicianList :many
SELECT id, name, sort_name FROM musicians ORDER BY sort_name ASC;

-- name: CreateMusician :one
INSERT INTO musicians (
    name,
    sort_name,
    spotify_id,
    spotify_popularity,
    spotify_followers,
    summary,
    thumb
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;
