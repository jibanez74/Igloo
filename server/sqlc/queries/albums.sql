-- name: GetAlbumByID :one
SELECT
  *
FROM
  albums
WHERE
  id = ?
LIMIT
  1;

-- name: GetAlbumByTitleAndMusician :one
SELECT * FROM albums WHERE title = ? AND musician = ? LIMIT 1;

-- name: GetAlbumByMusicBrainzID :one
SELECT
  *
FROM
  albums
WHERE
  musicbrainz_id = ?
LIMIT
  1;

-- name: GetLatestAlbums :many
SELECT
  id,
  title,
  cover,
  musician,
  year
FROM
  albums
ORDER BY
  created_at DESC
LIMIT
  12;

-- name: GetAlbumsAlphabetical :many
-- Returns albums sorted alphabetically by title with pagination.
-- Non-alphabetic titles (numbers, symbols) are grouped under '#' and sorted first.
SELECT
  id,
  title,
  cover,
  musician,
  year
FROM
  albums
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(title, 1, 1)) BETWEEN 'A' AND 'Z'
    THEN UPPER(SUBSTR(title, 1, 1))
    ELSE '#'
  END,
  UPPER(title)
LIMIT ? OFFSET ?;

-- name: UpsertAlbum :one
INSERT INTO
  albums (
    title,
    sort_title,
    musician,
    musicbrainz_id,
    release_date,
    year,
    total_tracks,
    cover
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (title, musician) DO
UPDATE
SET
  sort_title = excluded.sort_title,
  musicbrainz_id = COALESCE(excluded.musicbrainz_id, albums.musicbrainz_id),
  release_date = COALESCE(excluded.release_date, albums.release_date),
  year = COALESCE(excluded.year, albums.year),
  total_tracks = COALESCE(excluded.total_tracks, albums.total_tracks),
  cover = COALESCE(excluded.cover, albums.cover),
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: UpdateAlbumCover :exec
UPDATE albums SET cover = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: GetAlbumsWithMissingCovers :many
SELECT * FROM albums
WHERE musicbrainz_id IS NOT NULL AND musicbrainz_id != ''
  AND (cover IS NULL OR cover = '');

-- name: DeleteAlbum :exec
DELETE FROM albums WHERE id = ?;
