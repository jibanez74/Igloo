-- name: CreateDevice :one
INSERT INTO devices (
  user_id,
  name,
  platform,
  app_version,
  token_hash
)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetDeviceByTokenHash :one
SELECT *
FROM devices
WHERE token_hash = ?
LIMIT 1;

-- name: GetDevicesByUser :many
SELECT
  id,
  user_id,
  name,
  platform,
  app_version,
  created_at,
  last_used_at
FROM devices
WHERE user_id = ?
ORDER BY last_used_at DESC, id DESC;

-- name: UpdateDeviceLastUsed :exec
UPDATE devices
SET last_used_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: RenameDevice :execrows
UPDATE devices
SET name = sqlc.arg(name)
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id);

-- name: DeleteDeviceForUser :execrows
DELETE FROM devices
WHERE id = sqlc.arg(id)
  AND user_id = sqlc.arg(user_id);
