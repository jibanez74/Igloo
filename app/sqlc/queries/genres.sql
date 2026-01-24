-- name: GetOrCreateGenre :one
INSERT INTO
  genres (tag, genre_type)
VALUES
  (?, ?) ON CONFLICT (tag, genre_type) DO
UPDATE
SET
  updated_at = CURRENT_TIMESTAMP RETURNING *;

-- name: UpsertMusicianGenre :exec
-- Creates a relationship between a musician and a genre (idempotent)
INSERT INTO musician_genres (musician_id, genre_id)
VALUES (?, ?)
ON CONFLICT (musician_id, genre_id) DO NOTHING;

-- name: UpsertAlbumGenre :exec
-- Creates a relationship between an album and a genre (idempotent)
INSERT INTO album_genres (album_id, genre_id)
VALUES (?, ?)
ON CONFLICT (album_id, genre_id) DO NOTHING;

-- name: GetGenresByMusicianID :many
-- Returns all genres associated with a musician
SELECT
  g.id,
  g.tag
FROM
  genres g
  INNER JOIN musician_genres mg ON g.id = mg.genre_id
WHERE
  mg.musician_id = ?
ORDER BY
  g.tag ASC;

-- name: GetGenresByAlbumIDDirect :many
-- Returns genres directly associated with an album via album_genres table
SELECT
  g.id,
  g.tag
FROM
  genres g
  INNER JOIN album_genres ag ON g.id = ag.genre_id
WHERE
  ag.album_id = ?
ORDER BY
  g.tag ASC;