-- name: GetUser :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: GetAdminUser :one
SELECT
  *
FROM
  users
WHERE
  is_admin = true
LIMIT
  1;

-- name: GetUserByEmail :one
SELECT
  *
FROM
  users
WHERE
  email = ?
LIMIT
  1;

-- name: CreateUser :one
INSERT INTO
  users (name, email, password, is_admin, avatar)
VALUES
  (?, ?, ?, ?, ?) RETURNING *;

-- name: UpdateUserName :one
UPDATE users SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateUserAvatar :one
UPDATE users SET avatar = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;