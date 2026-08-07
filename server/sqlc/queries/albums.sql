-- name: GetAlbumByID :one
SELECT
  *
FROM albums
WHERE id = ?
LIMIT 1;

-- name: GetAlbumByKey :one
SELECT
  *
FROM albums
WHERE album_key = ?
LIMIT 1;

-- name: GetLatestAlbums :many
SELECT
  id,
  title,
  cover,
  musician,
  year
FROM albums
ORDER BY created_at DESC
LIMIT 12;

-- name: GetAlbumsAlphabetical :many
-- Returns albums sorted alphabetically by title with pagination.
-- Non-alphabetic titles (numbers, symbols) are grouped under '#' and sorted first.
SELECT
  id,
  title,
  cover,
  musician,
  year
FROM albums
ORDER BY
  CASE
    WHEN UPPER(SUBSTR(title, 1, 1)) BETWEEN 'A' AND 'Z' THEN UPPER(SUBSTR(title, 1, 1))
    ELSE '#'
  END,
  UPPER(title)
LIMIT ?
OFFSET ?;

-- name: UpsertAlbum :one
-- Tag-owned fields only; enrichment columns (cover, audiodb id) are written by
-- UpdateAlbumEnrichment. title and musician are display strings, deliberately
-- absent from the update list (first scanned spelling wins); identity lives
-- entirely in album_key. Date and track-count fields prefer the incoming tag
-- but keep an existing value when the new track lacks one, since only some
-- tracks of an album carry them. is_compilation is sticky-true so an
-- enrichment pass that flags a compilation is never undone by a rescan.
INSERT INTO albums (
  title,
  sort_title,
  album_key,
  album_artist_id,
  musician,
  is_compilation,
  mb_release_group_id,
  mb_release_id,
  release_date,
  year,
  total_tracks
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (album_key) DO UPDATE
SET
  sort_title = excluded.sort_title,
  album_artist_id = COALESCE(albums.album_artist_id, excluded.album_artist_id),
  is_compilation = albums.is_compilation OR excluded.is_compilation,
  mb_release_group_id = COALESCE(albums.mb_release_group_id, excluded.mb_release_group_id),
  mb_release_id = COALESCE(albums.mb_release_id, excluded.mb_release_id),
  release_date = COALESCE(excluded.release_date, albums.release_date),
  year = COALESCE(excluded.year, albums.year),
  total_tracks = COALESCE(excluded.total_tracks, albums.total_tracks),
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: DeleteAlbum :exec
DELETE FROM albums
WHERE id = ?;
