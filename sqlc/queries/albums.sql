-- name: GetAlbumCount :one
SELECT COUNT(*) FROM albums;

-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: GetAlbumByTitle :one
SELECT * FROM albums Where title = $1;

-- name: GetLatestAlbums :many
SELECT 
    id,
    title,
    musician,
    cover
FROM albums
ORDER BY created_at DESC
LIMIT 12;

-- name: GetAlbumDetails :one
WITH album_base AS (
    SELECT a.id, a.created_at, a.updated_at, a.title, a.sort_title, a.spotify_id, a.release_date, a.year, a.spotify_popularity, a.total_tracks, a.musician, a.cover
    FROM albums a
    WHERE a.id = $1
),
album_genres_cte AS (
    SELECT COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', g.id,
            'tag', g.tag
        )) FILTER (WHERE g.id IS NOT NULL), '[]'
    ) as genres
    FROM album_base a
    LEFT JOIN album_genres ag ON ag.album_id = a.id
    LEFT JOIN genres g ON g.id = ag.genre_id
),
album_musicians_cte AS (
    SELECT COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', m.id,
            'name', m.name,
            'sort_name', m.sort_name,
            'thumb', m.thumb,
            'spotify_id', m.spotify_id
        )) FILTER (WHERE m.id IS NOT NULL), '[]'
    ) as musicians
    FROM album_base a
    LEFT JOIN album_musicians am ON am.album_id = a.id
    LEFT JOIN musicians m ON m.id = am.musician_id
),
album_tracks AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', t.id,
                'title', t.title,
                'sort_title', t.sort_title,
                'disc', t.disc,
                'track_index', t.track_index,
                'duration', t.duration,
                'file_path', t.file_path,
                'file_name', t.file_name,
                'container', t.container,
                'codec', t.codec,
                'channels', t.channels,
                'channel_layout', t.channel_layout,
                'size', t.size,
                'bit_rate', t.bit_rate
            )
            ORDER BY t.disc, t.track_index
        ) FILTER (WHERE t.id IS NOT NULL), '[]'
    ) as tracks
    FROM album_base a
    LEFT JOIN tracks t ON t.album_id = a.id
)
SELECT 
    a.id, a.created_at, a.updated_at, a.title, a.sort_title, a.spotify_id, a.release_date, a.year, a.spotify_popularity, a.total_tracks, a.musician, a.cover,
    g.genres,
    m.musicians,
    t.tracks
FROM album_base a
CROSS JOIN album_genres_cte g
CROSS JOIN album_musicians_cte m
CROSS JOIN album_tracks t;

-- name: CreateAlbum :one
INSERT INTO albums (
    title,
    sort_title,
    spotify_id,
    release_date,
    year,
    spotify_popularity,
    total_tracks,
    musician,
    cover
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;
