-- name: GetMusiciansCount :one
SELECT COUNT(*) FROM musicians;

-- name: GetAllMusiciansWithImages :many
SELECT 
    m.id, 
    m.name,
    si.id as image_id, 
    si.path as image_path, 
    si.width as image_width, 
    si.height as image_height
FROM musicians m
LEFT JOIN spotify_images si ON m.id = si.musician_id
ORDER BY m.name ASC;

-- name: CreateMusician :one
INSERT INTO musicians (
    name,
    spotify_id,
    spotify_popularity,
    spotify_followers
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: CheckMusicianExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM musicians WHERE spotify_id = $1
) as exists;
