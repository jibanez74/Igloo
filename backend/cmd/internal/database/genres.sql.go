// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: genres.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const getGenreByID = `-- name: GetGenreByID :one
SELECt tag FROM genres
WHERE id = $1
`

func (q *Queries) GetGenreByID(ctx context.Context, id int32) (string, error) {
	row := q.db.QueryRow(ctx, getGenreByID, id)
	var tag string
	err := row.Scan(&tag)
	return tag, err
}

const getOrCreateGenre = `-- name: GetOrCreateGenre :one
WITH existing_genre AS (
    SELECT g.id, g.created_at, g.updated_at, g.tag, g.genre_type
    FROM genres g
    WHERE g.tag = $1
    LIMIT 1
), new_genre AS (
    INSERT INTO genres (
        tag,
        genre_type
    )
    SELECT $1, $2
    WHERE NOT EXISTS (SELECT 1 FROM existing_genre)
    RETURNING id, created_at, updated_at, tag, genre_type
)
SELECT e.id, e.created_at, e.updated_at, e.tag, e.genre_type
FROM existing_genre e
UNION ALL
SELECT n.id, n.created_at, n.updated_at, n.tag, n.genre_type
FROM new_genre n
`

type GetOrCreateGenreParams struct {
	Tag       string `json:"tag"`
	GenreType string `json:"genre_type"`
}

type GetOrCreateGenreRow struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	Tag       string             `json:"tag"`
	GenreType string             `json:"genre_type"`
}

func (q *Queries) GetOrCreateGenre(ctx context.Context, arg GetOrCreateGenreParams) (GetOrCreateGenreRow, error) {
	row := q.db.QueryRow(ctx, getOrCreateGenre, arg.Tag, arg.GenreType)
	var i GetOrCreateGenreRow
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Tag,
		&i.GenreType,
	)
	return i, err
}
