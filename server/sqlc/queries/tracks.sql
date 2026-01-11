-- name: GetTrack :one
SELECT * FROM tracks WHERE id = ? LIMIT 1;

-- name: CheckTrackUnchanged :one
-- Quick check if track exists with same path and size (likely unchanged)
SELECT 1 FROM tracks WHERE file_path = ? AND size = ? LIMIT 1;

-- name: UpsertTrack :one
INSERT INTO tracks (
  title, sort_title, file_path, file_name, container, mime_type, codec, size,
  track_index, duration, disc, channels, channel_layout, bit_rate, profile,
  release_date, year, composer, copyright, language, album_id, musician_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (file_path) DO UPDATE SET
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
  release_date = COALESCE(excluded.release_date, tracks.release_date),
  year = COALESCE(excluded.year, tracks.year),
  composer = COALESCE(excluded.composer, tracks.composer),
  copyright = COALESCE(excluded.copyright, tracks.copyright),
  language = COALESCE(excluded.language, tracks.language),
  album_id = COALESCE(excluded.album_id, tracks.album_id),
  musician_id = COALESCE(excluded.musician_id, tracks.musician_id),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetTracksByAlbumID :many
SELECT
  *
FROM
  tracks
WHERE
  album_id = ?
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
  a.id as album_id,
  a.title as album_title,
  m.id as musician_id,
  m.name as musician_name
FROM tracks t
LEFT JOIN albums a ON t.album_id = a.id
LEFT JOIN musicians m ON t.musician_id = m.id
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(t.title, 1, 1)) BETWEEN 'A' AND 'Z'
    THEN UPPER(SUBSTR(t.title, 1, 1))
    ELSE '#'
  END,
  UPPER(t.title)
LIMIT ? OFFSET ?;

-- name: GetTracksCount :one
SELECT COUNT(*) FROM tracks;

-- name: GetAlbumsCount :one
SELECT COUNT(*) FROM albums;

-- name: GetMusiciansCount :one
SELECT COUNT(*) FROM musicians;
