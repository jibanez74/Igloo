-- name: GetTotalUsersCount :one
SELECT COUNT(*) FROM users;

-- name: GetUsersPaginated :many
SELECT 
    id,
    name,
    email,
    is_active,
    is_admin
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetUserForLogin :one
SELECT id, name, email, password, is_admin, avatar FROM users WHERE email = $1 AND is_active = true;

-- name: GetUserByID :one
SELECT id, created_at, updated_at, name, email, is_active, is_admin, avatar 
FROM users 
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (
    name,
    email,
    password,
    is_admin,
    is_active,
    avatar
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;
