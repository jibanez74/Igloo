-- name: GetOrCreateGenre :one
INSERT INTO genres (
  tag,
  genre_type
)
VALUES
  (?, ?)
ON CONFLICT (tag, genre_type) DO UPDATE
SET
  updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetGenresByMusicianID :many
-- Returns all genres associated with a musician
SELECT DISTINCT
  g.id,
  g.tag
FROM genres AS g
INNER JOIN musician_genres AS mg
  ON g.id = mg.genre_id
WHERE mg.musician_id = ?
ORDER BY g.tag ASC;

-- name: GetAlbumGenres :many
-- Returns all genres associated with an album
SELECT DISTINCT
  g.id,
  g.tag
FROM genres AS g
INNER JOIN album_genres AS ag
  ON g.id = ag.genre_id
WHERE ag.album_id = ?
ORDER BY g.tag ASC;
