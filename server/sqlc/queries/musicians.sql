-- name: GetMusicianByName :one
SELECT * FROM musicians WHERE name = ? LIMIT 1;

-- name: GetMusicianByMusicBrainzID :one
SELECT * FROM musicians WHERE musicbrainz_id = ? LIMIT 1;

-- name: UpsertMusician :one
INSERT INTO musicians (name, sort_name, summary, musicbrainz_id, thumb)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (name) DO UPDATE SET
  sort_name = excluded.sort_name,
  summary = COALESCE(excluded.summary, musicians.summary),
  musicbrainz_id = COALESCE(excluded.musicbrainz_id, musicians.musicbrainz_id),
  thumb = COALESCE(excluded.thumb, musicians.thumb),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetMusiciansByAlbumID :many
SELECT
  m.id,
  m.name,
  m.thumb,
  m.musicbrainz_id
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

-- name: UpdateMusicianThumb :exec
UPDATE musicians SET thumb = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: GetMusicianByID :one
SELECT * FROM musicians WHERE id = ? LIMIT 1;

-- name: GetAlbumsByMusicianID :many
-- Sorted by release date (newest first), then by title
SELECT
  a.id,
  a.title,
  a.cover,
  a.year,
  a.release_date,
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
