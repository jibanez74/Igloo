-- name: GetMusicianBySpotifyID :one
SELECT
  *
FROM musicians
WHERE spotify_id = ?
LIMIT 1;

-- name: GetMusicianByName :one
SELECT
  *
FROM musicians
WHERE name = ?
LIMIT 1;

-- name: UpdateMusicianSpotifyThumb :one
UPDATE musicians
SET
  thumb = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpsertMusician :one
INSERT INTO musicians (
  name,
  sort_name,
  summary,
  spotify_id,
  spotify_popularity,
  spotify_followers,
  thumb
)
VALUES
  (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (name) DO UPDATE
SET
  sort_name = excluded.sort_name,
  summary = COALESCE(excluded.summary, musicians.summary),
  spotify_id = COALESCE(excluded.spotify_id, musicians.spotify_id),
  spotify_popularity = COALESCE(excluded.spotify_popularity, musicians.spotify_popularity),
  spotify_followers = COALESCE(excluded.spotify_followers, musicians.spotify_followers),
  thumb = COALESCE(excluded.thumb, musicians.thumb),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

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
SELECT
  m.id,
  m.name,
  m.thumb,
  m.sort_name,
  (
    SELECT COUNT(*)
    FROM musician_albums AS ma
    WHERE ma.musician_id = m.id
  ) AS album_count,
  -- Counts tracks credited to this musician either as the primary musician or via
  -- track_musicians. A UNION of two indexed lookups rather than an OR over both
  -- tables: the OR form cannot use an index and scans every track per musician.
  -- UNION (not UNION ALL) preserves the distinct-track count.
  (
    SELECT COUNT(*)
    FROM (
      SELECT t.id
      FROM tracks AS t
      WHERE t.musician_id = m.id
      UNION
      SELECT tm.track_id
      FROM track_musicians AS tm
      WHERE tm.musician_id = m.id
    )
  ) AS track_count
FROM musicians AS m
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(m.sort_name, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(m.sort_name, 1, 1))
    ELSE '#'
  END,
  m.sort_name
LIMIT ?
OFFSET ?;

-- name: GetMusicianByID :one
SELECT
  *
FROM musicians
WHERE id = ?
LIMIT 1;

-- name: GetAlbumsByMusicianID :many
-- Sorted by release date (newest first), then by title. Track counts come from
-- one grouped pass over idx_track_album instead of a correlated subquery per
-- album; this query has no LIMIT, so it runs for the artist's whole
-- discography.
SELECT
  a.id,
  a.title,
  a.cover,
  a.year,
  a.release_date,
  COALESCE(tc.track_count, 0) AS track_count
FROM albums AS a
INNER JOIN musician_albums AS ma
  ON a.id = ma.album_id
LEFT JOIN (
  SELECT
    album_id,
    COUNT(*) AS track_count
  FROM tracks
  GROUP BY album_id
) AS tc
  ON tc.album_id = a.id
WHERE ma.musician_id = ?
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
  a.id AS album_id,
  a.title AS album_title,
  a.cover AS album_cover
FROM tracks AS t
LEFT JOIN albums AS a
  ON t.album_id = a.id
-- Same UNION-of-indexed-lookups shape as GetMusiciansAlphabetical's track_count:
-- the equivalent OR over tracks and track_musicians cannot use an index.
WHERE t.id IN (
  SELECT primary_t.id
  FROM tracks AS primary_t
  WHERE primary_t.musician_id = sqlc.arg(musician_id)
  UNION
  SELECT credited_tm.track_id
  FROM track_musicians AS credited_tm
  WHERE credited_tm.musician_id = sqlc.arg(musician_id)
)
ORDER BY t.sort_title ASC;
