-- name: GetAlbumByID :one
SELECT
  *
FROM
  albums
WHERE
  id = ?
LIMIT
  1;

-- name: GetAlbumBySpotifyID :one
SELECT
  *
FROM
  albums
WHERE
  spotify_id = ?
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
    spotify_id,
    spotify_popularity,
    release_date,
    year,
    total_tracks,
    cover
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (title, musician) DO
UPDATE
SET
  sort_title = excluded.sort_title,
  spotify_id = COALESCE(excluded.spotify_id, albums.spotify_id),
  spotify_popularity = COALESCE(
    excluded.spotify_popularity,
    albums.spotify_popularity
  ),
  release_date = COALESCE(excluded.release_date, albums.release_date),
  year = COALESCE(excluded.year, albums.year),
  total_tracks = COALESCE(excluded.total_tracks, albums.total_tracks),
  cover = COALESCE(excluded.cover, albums.cover),
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: DeleteAlbum :exec
-- Deleting an album will cascade delete all associated tracks
DELETE FROM albums WHERE id = ?;