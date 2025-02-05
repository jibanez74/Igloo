// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: audio_streams.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createAudioStream = `-- name: CreateAudioStream :one
INSERT INTO audio_streams (
    title,
    index,
    profile,
    codec,
    channels,
    channel_layout,
    language,
    movie_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, created_at, updated_at, title, index, profile, codec, channels, channel_layout, language, movie_id
`

type CreateAudioStreamParams struct {
	Title         string      `json:"title"`
	Index         int32       `json:"index"`
	Profile       string      `json:"profile"`
	Codec         string      `json:"codec"`
	Channels      int32       `json:"channels"`
	ChannelLayout string      `json:"channel_layout"`
	Language      string      `json:"language"`
	MovieID       pgtype.Int4 `json:"movie_id"`
}

func (q *Queries) CreateAudioStream(ctx context.Context, arg CreateAudioStreamParams) (AudioStream, error) {
	row := q.db.QueryRow(ctx, createAudioStream,
		arg.Title,
		arg.Index,
		arg.Profile,
		arg.Codec,
		arg.Channels,
		arg.ChannelLayout,
		arg.Language,
		arg.MovieID,
	)
	var i AudioStream
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Title,
		&i.Index,
		&i.Profile,
		&i.Codec,
		&i.Channels,
		&i.ChannelLayout,
		&i.Language,
		&i.MovieID,
	)
	return i, err
}
