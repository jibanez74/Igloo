-- name: GetOrCreateGenre :one
INSERT INTO
  genres (tag, genre_type)
VALUES
  (?, ?) ON CONFLICT (tag, genre_type) DO
UPDATE
SET
  updated_at = CURRENT_TIMESTAMP RETURNING *;