-- name: GetTotalUsersCount :one
SELECT COUNT(*) FROM users;

-- name: GetUsersPaginated :many
SELECT 
    id,
    name,
    email,
    username,
    is_active,
    is_admin
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

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

-- name: CheckUserExists :one
SELECT EXISTS (
    SELECT 1 FROM users
    WHERE email = $1 OR username = $2
);

-- name: UpdateUserAvatar :one
UPDATE users
SET avatar = $1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING id, name, email, username, is_active, is_admin, avatar;

-- name: DeleteUser :exec
DELETE FROM users 
WHERE users.id = $1 
AND users.id != (
    -- Prevent deletion of the last admin user
    SELECT users.id FROM users 
    WHERE users.is_admin = true 
    ORDER BY users.created_at ASC 
    LIMIT 1
);
