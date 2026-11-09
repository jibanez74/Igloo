-- name: GetMusicianBySpotifyID :one
SELECT id, spotify_id, thumb_source
FROM musicians
WHERE spotify_id = ?
LIMIT 1;

-- name: UpdateMusicianSpotifyThumb :one
UPDATE musicians
SET
  thumb_source = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, spotify_id, thumb_source;

-- name: UpsertMusician :one
INSERT INTO musicians (
  name,
  sort_name,
  spotify_id,
  thumb_source
)
VALUES
  (?, ?, ?, ?)
ON CONFLICT (name) DO UPDATE
SET
  sort_name = excluded.sort_name,
  spotify_id = COALESCE(excluded.spotify_id, musicians.spotify_id),
  thumb_source = COALESCE(excluded.thumb_source, musicians.thumb_source),
  updated_at = CURRENT_TIMESTAMP
RETURNING id, spotify_id, thumb_source;

-- name: GetMusiciansByAlbumID :many
SELECT
  m.id,
  m.name,
  m.thumb
FROM musicians AS m
INNER JOIN musician_albums AS ma
  ON m.id = ma.musician_id
WHERE ma.album_id = ?
ORDER BY m.name ASC;

-- name: GetMusiciansAlphabetical :many
-- Returns musicians sorted alphabetically by sort_name with pagination.
-- Non-alphabetic names (numbers, symbols) are grouped under '#' and sorted first.
-- sort_name is not unique, so id breaks ties and pages stay stable.
SELECT
  m.id,
  m.name,
  m.thumb,
  (
    SELECT COUNT(*)
    FROM musician_albums AS ma
    WHERE ma.musician_id = m.id
  ) AS album_count,
  -- Every credit has a track_musicians row, the primary one included: the
  -- scanner writes tracks.musician_id and that link in one transaction, so the
  -- link table alone counts each credited track once.
  -- searchMusiciansSQL in cmd/api/search_handler.go must keep this identical: it
  -- scans into this same row type, so a divergence shows the same artist card
  -- with two different track counts.
  (
    SELECT COUNT(*)
    FROM track_musicians AS tm
    WHERE tm.musician_id = m.id
  ) AS track_count
FROM musicians AS m
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(m.sort_name, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(m.sort_name, 1, 1))
    ELSE '#'
  END,
  m.sort_name,
  m.id
LIMIT ?
OFFSET ?;

-- name: GetMusicianByID :one
SELECT
  *
FROM musicians
WHERE id = ?
LIMIT 1;

-- name: GetAlbumsByMusicianID :many
-- Sorted by release date (newest first), then by title. The track count is a
-- correlated subquery: one probe of idx_track_album per album in the artist's
-- discography, which is tens of rows. The grouped-pass form that lived here
-- was uncorrelated, so SQLite materialized it by scanning every track in the
-- library on each artist-detail request.
SELECT
  a.id,
  a.title,
  a.cover,
  a.year,
  a.release_date,
  (
    SELECT COUNT(*)
    FROM tracks AS t
    WHERE t.album_id = a.id
  ) AS track_count
FROM albums AS a
INNER JOIN musician_albums AS ma
  ON a.id = ma.album_id
WHERE ma.musician_id = ?
ORDER BY
  a.release_date DESC,
  a.year DESC,
  a.sort_title ASC;

-- name: GetTracksByMusicianID :many
-- Reads the credits from track_musicians alone, like GetMusiciansAlphabetical's
-- track_count, so the list and the count on the artist's card agree.
SELECT
  t.id,
  t.title,
  t.duration,
  t.codec,
  t.bit_rate,
  a.id AS album_id,
  a.title AS album_title,
  a.cover AS album_cover
FROM track_musicians AS tm
INNER JOIN tracks AS t
  ON t.id = tm.track_id
LEFT JOIN albums AS a
  ON t.album_id = a.id
WHERE tm.musician_id = ?
ORDER BY
  t.sort_title ASC,
  t.id ASC;
