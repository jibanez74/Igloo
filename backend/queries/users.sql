-- name: GetUserByID :one
SELECT id, name, email, username, is_active, is_admin, avatar FROM users
WHERE id = $1;

-- name: GetUserForLogin :one
SELECT * FROM users
WHERE email = $1 
AND username = $2
AND is_active = true;

-- name: CreateUser :one
INSERT INTO users (
    name,
    email,
    username,
    password,
    is_active,
    is_admin,
    avatar
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;
