// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: users.sql

package database

import (
	"context"
)

const createUser = `-- name: CreateUser :one
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
RETURNING id, created_at, updated_at, name, email, username, password, is_active, is_admin, avatar
`

type CreateUserParams struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	IsActive bool   `json:"is_active"`
	IsAdmin  bool   `json:"is_admin"`
	Avatar   string `json:"avatar"`
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRow(ctx, createUser,
		arg.Name,
		arg.Email,
		arg.Username,
		arg.Password,
		arg.IsActive,
		arg.IsAdmin,
		arg.Avatar,
	)
	var i User
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Name,
		&i.Email,
		&i.Username,
		&i.Password,
		&i.IsActive,
		&i.IsAdmin,
		&i.Avatar,
	)
	return i, err
}

const getTotalUsersCount = `-- name: GetTotalUsersCount :one
SELECT COUNT(*) FROM users
`

func (q *Queries) GetTotalUsersCount(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, getTotalUsersCount)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getUserByID = `-- name: GetUserByID :one
SELECT id, name, email, username, is_active, is_admin, avatar FROM users
WHERE id = $1
`

type GetUserByIDRow struct {
	ID       int32  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
	IsAdmin  bool   `json:"is_admin"`
	Avatar   string `json:"avatar"`
}

func (q *Queries) GetUserByID(ctx context.Context, id int32) (GetUserByIDRow, error) {
	row := q.db.QueryRow(ctx, getUserByID, id)
	var i GetUserByIDRow
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Email,
		&i.Username,
		&i.IsActive,
		&i.IsAdmin,
		&i.Avatar,
	)
	return i, err
}

const getUserForLogin = `-- name: GetUserForLogin :one
SELECT id, created_at, updated_at, name, email, username, password, is_active, is_admin, avatar FROM users
WHERE email = $1 
AND username = $2
AND is_active = true
`

type GetUserForLoginParams struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (q *Queries) GetUserForLogin(ctx context.Context, arg GetUserForLoginParams) (User, error) {
	row := q.db.QueryRow(ctx, getUserForLogin, arg.Email, arg.Username)
	var i User
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Name,
		&i.Email,
		&i.Username,
		&i.Password,
		&i.IsActive,
		&i.IsAdmin,
		&i.Avatar,
	)
	return i, err
}

const getUsersPaginated = `-- name: GetUsersPaginated :many
SELECT 
    id,
    name,
    email,
    username,
    is_active,
    is_admin
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2
`

type GetUsersPaginatedParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

type GetUsersPaginatedRow struct {
	ID       int32  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
	IsAdmin  bool   `json:"is_admin"`
}

func (q *Queries) GetUsersPaginated(ctx context.Context, arg GetUsersPaginatedParams) ([]GetUsersPaginatedRow, error) {
	rows, err := q.db.Query(ctx, getUsersPaginated, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GetUsersPaginatedRow{}
	for rows.Next() {
		var i GetUsersPaginatedRow
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Email,
			&i.Username,
			&i.IsActive,
			&i.IsAdmin,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
