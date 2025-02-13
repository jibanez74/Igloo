// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: movie_extras.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createMovieExtra = `-- name: CreateMovieExtra :one
INSERT INTO movie_extras (
    title,
    url,
    kind,
    movie_id
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, created_at, updated_at, title, url, kind, movie_id
`

type CreateMovieExtraParams struct {
	Title   string      `json:"title"`
	Url     string      `json:"url"`
	Kind    string      `json:"kind"`
	MovieID pgtype.Int4 `json:"movie_id"`
}

func (q *Queries) CreateMovieExtra(ctx context.Context, arg CreateMovieExtraParams) (MovieExtra, error) {
	row := q.db.QueryRow(ctx, createMovieExtra,
		arg.Title,
		arg.Url,
		arg.Kind,
		arg.MovieID,
	)
	var i MovieExtra
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Title,
		&i.Url,
		&i.Kind,
		&i.MovieID,
	)
	return i, err
}
