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