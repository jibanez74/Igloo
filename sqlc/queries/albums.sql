-- name: GetOrCreateAlbumByTitle :one
WITH existing_album AS (
    SELECT a.id, a.created_at, a.updated_at, a.title, a.spotify_id, a.release_date, a.spotify_popularity, a.total_tracks, a.total_available_tracks
    FROM albums a
    WHERE a.title = $1
    LIMIT 1
), new_album AS (
    INSERT INTO albums (
        title,
        spotify_id,
        release_date,
        spotify_popularity,
        total_tracks,
        total_available_tracks
    )
    SELECT $1, $2, $3, $4, $5, $6
    WHERE NOT EXISTS (SELECT 1 FROM existing_album)
    RETURNING id, created_at, updated_at, title, spotify_id, release_date, spotify_popularity, total_tracks, total_available_tracks
)
SELECT e.id, e.created_at, e.updated_at, e.title, e.spotify_id, e.release_date, e.spotify_popularity, e.total_tracks, e.total_available_tracks
FROM existing_album e
UNION ALL
SELECT n.id, n.created_at, n.updated_at, n.title, n.spotify_id, n.release_date, n.spotify_popularity, n.total_tracks, n.total_available_tracks
FROM new_album n;

-- name: GetAlbumBySpotifyID :one
SELECT id, created_at, updated_at, title, spotify_id, release_date, spotify_popularity, total_tracks, total_available_tracks 
FROM albums 
WHERE spotify_id = $1 
LIMIT 1;

-- name: GetAllAlbums :many
SELECT 
    a.id, a.title, a.release_date,
    si.id as image_id, si.path as image_path, si.width as image_width, si.height as image_height
FROM albums a
LEFT JOIN spotify_images si ON a.id = si.album_id
ORDER BY a.title ASC;

-- name: GetAlbumsCount :one
SELECT COUNT(*) FROM albums;

-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    spotify_id,
    release_date,
    spotify_popularity,
    total_tracks,
    total_available_tracks
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: UpdateAlbum :one
UPDATE albums SET
    title = $1,
    spotify_id = $2,
    release_date = $3,
    spotify_popularity = $4,
    total_tracks = $5,
    total_available_tracks = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $7
RETURNING *;

-- name: GetAllAlbumsWithImages :many
SELECT 
    a.id, 
    a.title, 
    a.year,
    si.id as image_id, 
    si.path as image_path, 
    si.width as image_width, 
    si.height as image_height
FROM albums a
LEFT JOIN spotify_images si ON a.id = si.album_id
ORDER BY a.title ASC;

-- name: CheckAlbumExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM albums WHERE spotify_id = $1
) as exists;
