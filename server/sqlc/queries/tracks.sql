-- name: GetTrack :one
SELECT
  *
FROM tracks
WHERE id = ?
LIMIT 1;

-- name: TrackExists :one
-- Existence probe for handlers that only need to 404 on an unknown track.
SELECT
  EXISTS (
    SELECT 1
    FROM tracks
    WHERE id = ?
  );

-- name: ListMusicTrackScanIndex :many
SELECT
  file_path,
  size
FROM tracks;

-- name: UpsertTrack :one
INSERT INTO tracks (
  title,
  sort_title,
  file_path,
  file_name,
  container,
  mime_type,
  codec,
  size,
  track_index,
  duration,
  disc,
  channels,
  channel_layout,
  bit_rate,
  profile,
  release_date,
  year,
  composer,
  copyright,
  language,
  album_id,
  musician_id
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (file_path) DO UPDATE
SET
  title = excluded.title,
  sort_title = excluded.sort_title,
  file_name = excluded.file_name,
  container = excluded.container,
  mime_type = excluded.mime_type,
  codec = excluded.codec,
  size = excluded.size,
  track_index = excluded.track_index,
  duration = excluded.duration,
  disc = excluded.disc,
  channels = excluded.channels,
  channel_layout = excluded.channel_layout,
  bit_rate = excluded.bit_rate,
  profile = excluded.profile,
  release_date = excluded.release_date,
  year = excluded.year,
  composer = excluded.composer,
  copyright = excluded.copyright,
  language = excluded.language,
  album_id = excluded.album_id,
  musician_id = excluded.musician_id,
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetTracksByAlbumID :many
SELECT
  *
FROM tracks
WHERE album_id = ?
ORDER BY
  disc ASC,
  track_index ASC;

-- name: GetTracksAlphabetical :many
SELECT
  t.id,
  t.title,
  t.duration,
  t.codec,
  t.bit_rate,
  t.file_path,
  a.id AS album_id,
  a.title AS album_title,
  a.cover AS album_cover,
  m.id AS musician_id,
  m.name AS musician_name
FROM tracks AS t
LEFT JOIN albums AS a
  ON t.album_id = a.id
LEFT JOIN musicians AS m
  ON t.musician_id = m.id
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(t.title, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(t.title, 1, 1))
    ELSE '#'
  END,
  UPPER(t.title)
LIMIT ?
OFFSET ?;

-- name: GetTracksCount :one
SELECT
  COUNT(*)
FROM tracks;

-- name: GetAlbumsCount :one
SELECT
  COUNT(*)
FROM albums;

-- name: GetMusiciansCount :one
SELECT
  COUNT(*)
FROM musicians;

-- name: GetRandomTracks :many
SELECT
  t.id,
  t.title,
  t.file_path,
  t.duration,
  t.codec,
  t.bit_rate,
  a.id AS album_id,
  a.title AS album_title,
  a.cover AS album_cover,
  m.id AS musician_id,
  m.name AS musician_name
FROM tracks AS t
LEFT JOIN albums AS a
  ON t.album_id = a.id
LEFT JOIN musicians AS m
  ON t.musician_id = m.id
ORDER BY RANDOM()
LIMIT ?;
