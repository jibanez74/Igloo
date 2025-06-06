// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: studios.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const getOrCreateStudio = `-- name: GetOrCreateStudio :one
WITH existing_studio AS (
    SELECT s.id, s.created_at, s.updated_at, s.name, s.country, s.logo, s.tmdb_id 
    FROM studios s
    WHERE s.tmdb_id = $1
    LIMIT 1
), new_studio AS (
    INSERT INTO studios (
        name,
        country,
        logo,
        tmdb_id
    )
    SELECT $2, $3, $4, $1
    WHERE NOT EXISTS (SELECT 1 FROM existing_studio)
    RETURNING id, created_at, updated_at, name, country, logo, tmdb_id
)
SELECT id, created_at, updated_at, name, country, logo, tmdb_id 
FROM existing_studio
UNION ALL
SELECT id, created_at, updated_at, name, country, logo, tmdb_id 
FROM new_studio
`

type GetOrCreateStudioParams struct {
	TmdbID  int32  `json:"tmdb_id"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Logo    string `json:"logo"`
}

type GetOrCreateStudioRow struct {
	ID        int32              `json:"id"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
	UpdatedAt pgtype.Timestamptz `json:"updated_at"`
	Name      string             `json:"name"`
	Country   string             `json:"country"`
	Logo      string             `json:"logo"`
	TmdbID    int32              `json:"tmdb_id"`
}

func (q *Queries) GetOrCreateStudio(ctx context.Context, arg GetOrCreateStudioParams) (GetOrCreateStudioRow, error) {
	row := q.db.QueryRow(ctx, getOrCreateStudio,
		arg.TmdbID,
		arg.Name,
		arg.Country,
		arg.Logo,
	)
	var i GetOrCreateStudioRow
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Name,
		&i.Country,
		&i.Logo,
		&i.TmdbID,
	)
	return i, err
}
