-- name: GetUserPreferences :one
SELECT
  trailers_enabled,
  trailers_count,
  trailers_source
FROM user_preferences
WHERE user_id = ?;

-- name: UpsertUserPreferences :one
INSERT INTO user_preferences (
  user_id,
  trailers_enabled,
  trailers_count,
  trailers_source,
  updated_at
)
VALUES
  (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (user_id) DO UPDATE
SET
  trailers_enabled = excluded.trailers_enabled,
  trailers_count = excluded.trailers_count,
  trailers_source = excluded.trailers_source,
  updated_at = CURRENT_TIMESTAMP
RETURNING
  trailers_enabled,
  trailers_count,
  trailers_source;
