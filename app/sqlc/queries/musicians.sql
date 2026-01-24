-- name: GetMusicianBySpotifyID :one
SELECT * FROM musicians WHERE spotify_id = ? LIMIT 1;

-- name: UpsertMusician :one
INSERT INTO musicians (name, sort_name, summary, spotify_popularity, spotify_followers, spotify_id, thumb)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (name) DO UPDATE SET
  sort_name = excluded.sort_name,
  summary = COALESCE(excluded.summary, musicians.summary),
  spotify_popularity = COALESCE(excluded.spotify_popularity, musicians.spotify_popularity),
  spotify_followers = COALESCE(excluded.spotify_followers, musicians.spotify_followers),
  spotify_id = COALESCE(excluded.spotify_id, musicians.spotify_id),
  thumb = COALESCE(excluded.thumb, musicians.thumb),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetMusiciansByAlbumID :many
SELECT
  m.id,
  m.name,
  m.thumb,
  m.spotify_id
FROM
  musicians m
  INNER JOIN musician_albums ma ON m.id = ma.musician_id
WHERE
  ma.album_id = ?
ORDER BY
  m.name ASC;

-- name: GetMusiciansAlphabetical :many
-- Returns musicians sorted alphabetically by sort_name with pagination.
-- Non-alphabetic names (numbers, symbols) are grouped under '#' and sorted first.
SELECT
  m.id,
  m.name,
  m.thumb,
  m.sort_name,
  (SELECT COUNT(*) FROM musician_albums ma WHERE ma.musician_id = m.id) as album_count,
  (SELECT COUNT(*) FROM tracks t WHERE t.musician_id = m.id) as track_count
FROM
  musicians m
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(m.sort_name, 1, 1)) BETWEEN 'A' AND 'Z'
    THEN UPPER(SUBSTR(m.sort_name, 1, 1))
    ELSE '#'
  END,
  m.sort_name
LIMIT ? OFFSET ?;

-- name: GetMusicianByID :one
-- Returns a single musician by ID with full details
SELECT * FROM musicians WHERE id = ? LIMIT 1;

-- name: GetAlbumsByMusicianID :many
-- Returns all albums associated with a musician via the musician_albums join table
-- Sorted by release date (newest first), then by title
SELECT
  a.id,
  a.title,
  a.cover,
  a.year,
  a.release_date,
  a.spotify_popularity,
  (SELECT COUNT(*) FROM tracks t WHERE t.album_id = a.id) as track_count
FROM
  albums a
  INNER JOIN musician_albums ma ON a.id = ma.album_id
WHERE
  ma.musician_id = ?
ORDER BY
  a.release_date DESC,
  a.year DESC,
  a.sort_title ASC;

-- name: GetTracksByMusicianID :many
-- Returns all tracks by a musician, sorted alphabetically by sort_title
SELECT
  t.id,
  t.title,
  t.sort_title,
  t.duration,
  t.codec,
  t.bit_rate,
  t.file_path,
  t.track_index,
  t.disc,
  a.id as album_id,
  a.title as album_title,
  a.cover as album_cover
FROM tracks t
LEFT JOIN albums a ON t.album_id = a.id
WHERE t.musician_id = ?
ORDER BY t.sort_title ASC;
