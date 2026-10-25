-- name: GetAlbumByID :one
SELECT
  *
FROM albums
WHERE id = ?
LIMIT 1;

-- name: GetAlbumBySpotifyID :one
SELECT id, spotify_id, cover
FROM albums
WHERE spotify_id = ?
LIMIT 1;

-- name: GetAlbumsBySpotifyIDs :many
-- Batch form of GetAlbumBySpotifyID for the Spotify album search results
-- mapper: it only needs the library id per match, not the whole album row.
SELECT
  id,
  spotify_id
FROM albums
WHERE spotify_id IN (sqlc.slice(spotify_ids));

-- name: GetAlbumsWithoutSpotifyIDByIdentityKeys :many
-- Name fallback for the same mapper: library albums without a Spotify id, by
-- the (title, artist) alias keys derived from a page of results. The pairs
-- arrive as a JSON array of two-element arrays, like GetRandomTracks's
-- exclusions, so the statement stays preparable; each pair is a primary-key
-- probe of music_album_identity.
SELECT i.title_key, i.artist_key, i.album_id
FROM json_each(CAST(sqlc.arg(identity_keys) AS TEXT)) AS k
JOIN music_album_identity AS i
  ON i.title_key = json_extract(k.value, '$[0]')
  AND i.artist_key = json_extract(k.value, '$[1]')
JOIN albums AS a
  ON a.id = i.album_id
WHERE a.spotify_id IS NULL;

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
  UPPER(title),
  id
LIMIT ?
OFFSET ?;

-- name: UpdateAlbumCover :one
UPDATE albums
SET
  cover = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING id, spotify_id, cover;

-- name: UpsertAlbum :one
INSERT INTO albums (
  title,
  sort_title,
  spotify_id,
  musician,
  release_date,
  year,
  total_tracks,
  cover
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?)
-- Matches idx_albums_title_musician, which treats a missing musician as '' so an
-- untagged album cannot be inserted twice.
ON CONFLICT (title, COALESCE(musician, '')) DO UPDATE
SET
  sort_title = excluded.sort_title,
  spotify_id = COALESCE(excluded.spotify_id, albums.spotify_id),
  release_date = COALESCE(excluded.release_date, albums.release_date),
  year = COALESCE(excluded.year, albums.year),
  total_tracks = COALESCE(excluded.total_tracks, albums.total_tracks),
  cover = COALESCE(excluded.cover, albums.cover),
  updated_at = CURRENT_TIMESTAMP
RETURNING id, spotify_id, cover;

-- name: DeleteAlbum :one
-- RETURNING doubles as the existence check: no row means no such album.
DELETE FROM albums
WHERE id = ?
RETURNING title;
