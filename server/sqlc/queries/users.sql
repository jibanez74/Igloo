-- name: GetUser :one
SELECT
  *
FROM users
WHERE id = ?
LIMIT 1;

-- name: GetUserIsAdmin :one
-- Narrow admin check for hot paths (middleware, polled notification counts);
-- avoids shipping the full user row with its password hash.
SELECT
  is_admin
FROM users
WHERE id = ?
LIMIT 1;

-- name: UserExists :one
SELECT
  EXISTS (
    SELECT 1
    FROM users
    WHERE id = ?
  );

-- name: GetUserPin :one
SELECT
  pin
FROM users
WHERE id = ?
LIMIT 1;

-- name: GetAdminUser :one
SELECT
  *
FROM users
WHERE is_admin = true
LIMIT 1;

-- name: GetUserByEmail :one
SELECT
  *
FROM users
WHERE email = ?
LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (
  name,
  email,
  password,
  is_admin,
  avatar
)
VALUES
  (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateUserName :one
UPDATE users
SET
  name = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateUserEmail :one
UPDATE users
SET
  email = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET
  password = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateUserPin :one
UPDATE users
SET
  pin = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateUserAvatar :one
UPDATE users
SET
  avatar = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: GetAllUsers :many
-- has_pin instead of the PIN itself: the admin listing only shows whether one
-- is set, so the values never leave the database.
SELECT
  id,
  name,
  email,
  is_admin,
  avatar,
  CAST((pin IS NOT NULL) AS BOOLEAN) AS has_pin,
  created_at,
  updated_at
FROM users
ORDER BY name ASC;

-- name: AdminUpdateUser :one
UPDATE users
SET
  name = ?,
  email = ?,
  is_admin = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: CountAdmins :one
SELECT COUNT(*)
FROM users
WHERE is_admin = true;

-- name: GetUserPlaybackPreferences :one
SELECT
  is_admin,
  preferred_hls_profile,
  download_mbps,
  preferred_audio_language,
  preferred_subtitle_language
FROM users
WHERE id = ?;

-- name: UpdateUserPlaybackPreferences :one
UPDATE users
SET
  preferred_hls_profile = ?,
  download_mbps = ?,
  preferred_audio_language = ?,
  preferred_subtitle_language = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING
  id,
  preferred_hls_profile,
  download_mbps,
  preferred_audio_language,
  preferred_subtitle_language;

-- name: GetUsersExcluding :many
SELECT
  id,
  name,
  email,
  avatar
FROM users
WHERE id != sqlc.arg(excluded_id)
  AND (
    sqlc.arg(search) = ''
    OR INSTR(LOWER(name), LOWER(sqlc.arg(search))) > 0
    OR INSTR(LOWER(email), LOWER(sqlc.arg(search))) > 0
  )
ORDER BY name ASC;
