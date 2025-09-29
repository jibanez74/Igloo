-- name: GetAlbumCount :one
SELECT COUNT(*) FROM albums;

-- name: CheckAlbumExistsBySpotifyID :one
SELECT EXISTS(
    SELECT 1 FROM albums WHERE spotify_id = $1
) as exists;

-- name: GetAlbumBySpotifyID :one
SELECT * FROM albums WHERE spotify_id = $1 LIMIT 1;

-- name: GetAlbumByTitle :one
SELECT * FROM albums Where title = $1;

-- name: GetAlbumDetails :one
WITH album_base AS (
    SELECT a.*
    FROM albums a
    WHERE a.id = $1
),
album_musician AS (
    SELECT COALESCE(
        jsonb_build_object(
            'id', m.id,
            'name', m.name,
            'sort_name', m.sort_name,
            'spotify_id', m.spotify_id,
            'spotify_popularity', m.spotify_popularity,
            'thumb', m.thumb
        ), 'null'
    ) as musician
    FROM album_base a
    LEFT JOIN musicians m ON m.id = a.musician_id
),
album_genres AS (
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
album_tracks AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', t.id,
                'title', t.title,
                'sort_title', t.sort_title,
                'track_index', t.track_index,
                'duration', t.duration,
                'composer', t.composer,
                'release_date', t.release_date,
                'file_path', t.file_path,
                'container', t.container,
                'codec', t.codec,
                'bit_rate', t.bit_rate,
                'channel_layout', t.channel_layout,
                'copyright', t.copyright,
                'size', t.size,
                'file_name', t.file_name,
                'disc', t.disc,
                'language', t.language,
                'profile', t.profile,
                'sample_rate', t.sample_rate,
                'genres', COALESCE(
                    (SELECT json_agg(DISTINCT jsonb_build_object(
                        'id', tg_genre.id,
                        'tag', tg_genre.tag
                    )) FROM track_genres tg 
                     JOIN genres tg_genre ON tg_genre.id = tg.genre_id 
                     WHERE tg.track_id = t.id), '[]'
                )
            )
            ORDER BY t.disc, t.track_index
        ) FILTER (WHERE t.id IS NOT NULL), '[]'
    ) as tracks
    FROM album_base a
    LEFT JOIN tracks t ON t.album_id = a.id
)
SELECT 
    a.*,
    m.musician,
    g.genres,
    t.tracks
FROM album_base a
CROSS JOIN album_musician m
CROSS JOIN album_genres g
CROSS JOIN album_tracks t;

-- name: GetAlbumsPaginated :many
SELECT 
    a.id,
    a.title,
    a.cover,
    m.name as musician_name
FROM albums a
LEFT JOIN musicians m ON m.id = a.musician_id
ORDER BY a.sort_title ASC
LIMIT $1 OFFSET $2;

-- name: GetLatestAlbums :many
SELECT DISTINCT ON (a.title)
    a.title,
    a.cover,
    m.name as musician_name,
    a.year
FROM albums a
LEFT JOIN musicians m ON m.id = a.musician_id
ORDER BY a.title, a.created_at DESC
LIMIT 12;

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
