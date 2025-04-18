// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: movies.sql

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const createMovie = `-- name: CreateMovie :one
INSERT INTO movies (
    title,
    file_path,
    file_name,
    container,
    size,
    content_type,
    run_time,
    adult,
    tag_line,
    summary,
    art,
    thumb,
    tmdb_id,
    imdb_id,
    year,
    release_date,
    budget,
    revenue,
    content_rating,
    audience_rating,
    critic_rating,
    spoken_languages
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22
)
RETURNING id, created_at, updated_at, title, file_path, file_name, container, size, content_type, run_time, adult, tag_line, summary, art, thumb, tmdb_id, imdb_id, year, release_date, budget, revenue, content_rating, audience_rating, critic_rating, spoken_languages
`

type CreateMovieParams struct {
	Title           string      `json:"title"`
	FilePath        string      `json:"file_path"`
	FileName        string      `json:"file_name"`
	Container       string      `json:"container"`
	Size            int64       `json:"size"`
	ContentType     string      `json:"content_type"`
	RunTime         int32       `json:"run_time"`
	Adult           bool        `json:"adult"`
	TagLine         string      `json:"tag_line"`
	Summary         string      `json:"summary"`
	Art             string      `json:"art"`
	Thumb           string      `json:"thumb"`
	TmdbID          string      `json:"tmdb_id"`
	ImdbID          string      `json:"imdb_id"`
	Year            int32       `json:"year"`
	ReleaseDate     pgtype.Date `json:"release_date"`
	Budget          int64       `json:"budget"`
	Revenue         int64       `json:"revenue"`
	ContentRating   string      `json:"content_rating"`
	AudienceRating  float32     `json:"audience_rating"`
	CriticRating    float32     `json:"critic_rating"`
	SpokenLanguages string      `json:"spoken_languages"`
}

func (q *Queries) CreateMovie(ctx context.Context, arg CreateMovieParams) (Movie, error) {
	row := q.db.QueryRow(ctx, createMovie,
		arg.Title,
		arg.FilePath,
		arg.FileName,
		arg.Container,
		arg.Size,
		arg.ContentType,
		arg.RunTime,
		arg.Adult,
		arg.TagLine,
		arg.Summary,
		arg.Art,
		arg.Thumb,
		arg.TmdbID,
		arg.ImdbID,
		arg.Year,
		arg.ReleaseDate,
		arg.Budget,
		arg.Revenue,
		arg.ContentRating,
		arg.AudienceRating,
		arg.CriticRating,
		arg.SpokenLanguages,
	)
	var i Movie
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Title,
		&i.FilePath,
		&i.FileName,
		&i.Container,
		&i.Size,
		&i.ContentType,
		&i.RunTime,
		&i.Adult,
		&i.TagLine,
		&i.Summary,
		&i.Art,
		&i.Thumb,
		&i.TmdbID,
		&i.ImdbID,
		&i.Year,
		&i.ReleaseDate,
		&i.Budget,
		&i.Revenue,
		&i.ContentRating,
		&i.AudienceRating,
		&i.CriticRating,
		&i.SpokenLanguages,
	)
	return i, err
}

const getLatestMovies = `-- name: GetLatestMovies :many
SELECT 
    id,
    title,
    thumb,
    year
FROM movies
ORDER BY created_at DESC
LIMIT 12
`

type GetLatestMoviesRow struct {
	ID    int32  `json:"id"`
	Title string `json:"title"`
	Thumb string `json:"thumb"`
	Year  int32  `json:"year"`
}

func (q *Queries) GetLatestMovies(ctx context.Context) ([]GetLatestMoviesRow, error) {
	rows, err := q.db.Query(ctx, getLatestMovies)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GetLatestMoviesRow{}
	for rows.Next() {
		var i GetLatestMoviesRow
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Thumb,
			&i.Year,
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

const getMovieByTmdbID = `-- name: GetMovieByTmdbID :one
SELECT id, tmdb_id FROM movies
WHERE tmdb_id = $1
`

type GetMovieByTmdbIDRow struct {
	ID     int32  `json:"id"`
	TmdbID string `json:"tmdb_id"`
}

func (q *Queries) GetMovieByTmdbID(ctx context.Context, tmdbID string) (GetMovieByTmdbIDRow, error) {
	row := q.db.QueryRow(ctx, getMovieByTmdbID, tmdbID)
	var i GetMovieByTmdbIDRow
	err := row.Scan(&i.ID, &i.TmdbID)
	return i, err
}

const getMovieCount = `-- name: GetMovieCount :one
SELECT COUNT(*) FROM movies
`

func (q *Queries) GetMovieCount(ctx context.Context) (int64, error) {
	row := q.db.QueryRow(ctx, getMovieCount)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getMovieDetails = `-- name: GetMovieDetails :one
WITH movie_base AS (
    SELECT m.id, m.created_at, m.updated_at, m.title, m.file_path, m.file_name, m.container, m.size, m.content_type, m.run_time, m.adult, m.tag_line, m.summary, m.art, m.thumb, m.tmdb_id, m.imdb_id, m.year, m.release_date, m.budget, m.revenue, m.content_rating, m.audience_rating, m.critic_rating, m.spoken_languages
    FROM movies m
    WHERE m.id = $1
),
movie_genres AS (
    SELECT COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', g.id,
            'tag', g.tag
        )) FILTER (WHERE g.id IS NOT NULL), '[]'
    ) as genres
    FROM movie_base m
    LEFT JOIN movie_genres mg ON mg.movie_id = m.id
    LEFT JOIN genres g ON g.id = mg.genre_id
),
movie_studios AS (
    SELECT COALESCE(
        json_agg(DISTINCT jsonb_build_object(
            'id', s.id,
            'name', s.name,
            'logo', s.logo
        )) FILTER (WHERE s.id IS NOT NULL), '[]'
    ) as studios
    FROM movie_base m
    LEFT JOIN movie_studios ms ON ms.movie_id = m.id
    LEFT JOIN studios s ON s.id = ms.studio_id
),
movie_cast AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', a.id,
                'name', a.name,
                'thumb', a.thumb,
                'character', cl.character,
                'sort_order', cl.sort_order
            )
            ORDER BY cl.sort_order
        ) FILTER (WHERE a.id IS NOT NULL), '[]'
    ) as cast
    FROM movie_base m
    LEFT JOIN cast_list cl ON cl.movie_id = m.id
    LEFT JOIN artists a ON a.id = cl.artist_id
),
movie_crew AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', a.id,
                'name', a.name,
                'thumb', a.thumb,
                'job', cw.job,
                'department', cw.department
            )
        ) FILTER (WHERE a.id IS NOT NULL), '[]'
    ) as crew
    FROM movie_base m
    LEFT JOIN crew_list cw ON cw.movie_id = m.id
    LEFT JOIN artists a ON a.id = cw.artist_id
),
movie_extras AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', me.id,
                'title', me.title,
                'url', me.url,
                'kind', me.kind
            )
        ) FILTER (WHERE me.id IS NOT NULL), '[]'
    ) as extras
    FROM movie_base m
    LEFT JOIN movie_extras me ON me.movie_id = m.id
),
movie_streams AS (
    SELECT 
        COALESCE(
            json_agg(
                jsonb_build_object(
                    'id', vs.id,
                    'title', vs.title,
                    'index', vs.index,
                    'profile', vs.profile,
                    'aspect_ratio', vs.aspect_ratio,
                    'bit_rate', vs.bit_rate,
                    'bit_depth', vs.bit_depth,
                    'codec', vs.codec,
                    'width', vs.width,
                    'height', vs.height,
                    'coded_width', vs.coded_width,
                    'coded_height', vs.coded_height,
                    'color_transfer', vs.color_transfer,
                    'color_primaries', vs.color_primaries,
                    'color_space', vs.color_space,
                    'color_range', vs.color_range,
                    'frame_rate', vs.frame_rate,
                    'avg_frame_rate', vs.avg_frame_rate,
                    'level', vs.level
                )
            ) FILTER (WHERE vs.id IS NOT NULL), '[]'
        ) as video_streams,
        COALESCE(
            json_agg(
                jsonb_build_object(
                    'id', aus.id,
                    'title', aus.title,
                    'index', aus.index,
                    'codec', aus.codec,
                    'channels', aus.channels,
                    'channel_layout', aus.channel_layout,
                    'language', aus.language
                )
            ) FILTER (WHERE aus.id IS NOT NULL), '[]'
        ) as audio_streams,
        COALESCE(
            json_agg(
                jsonb_build_object(
                    'id', sub.id,
                    'title', sub.title,
                    'index', sub.index,
                    'codec', sub.codec,
                    'language', sub.language
                )
            ) FILTER (WHERE sub.id IS NOT NULL), '[]'
        ) as subtitles
    FROM movie_base m
    LEFT JOIN video_streams vs ON vs.movie_id = m.id
    LEFT JOIN audio_streams aus ON aus.movie_id = m.id
    LEFT JOIN subtitles sub ON sub.movie_id = m.id
),
movie_chapters AS (
    SELECT COALESCE(
        json_agg(
            jsonb_build_object(
                'id', ch.id,
                'title', ch.title,
                'start_time_ms', ch.start_time_ms,
                'thumb', ch.thumb
            )
            ORDER BY ch.start_time_ms
        ) FILTER (WHERE ch.id IS NOT NULL), '[]'
    ) as chapters
    FROM movie_base m
    LEFT JOIN chapters ch ON ch.movie_id = m.id
)
SELECT 
    m.id, m.created_at, m.updated_at, m.title, m.file_path, m.file_name, m.container, m.size, m.content_type, m.run_time, m.adult, m.tag_line, m.summary, m.art, m.thumb, m.tmdb_id, m.imdb_id, m.year, m.release_date, m.budget, m.revenue, m.content_rating, m.audience_rating, m.critic_rating, m.spoken_languages,
    g.genres,
    s.studios,
    c.cast,
    cr.crew,
    e.extras,
    ms.video_streams,
    ms.audio_streams,
    ms.subtitles,
    ch.chapters
FROM movie_base m
CROSS JOIN movie_genres g
CROSS JOIN movie_studios s
CROSS JOIN movie_cast c
CROSS JOIN movie_crew cr
CROSS JOIN movie_extras e
CROSS JOIN movie_streams ms
CROSS JOIN movie_chapters ch
`

type GetMovieDetailsRow struct {
	ID              int32              `json:"id"`
	CreatedAt       pgtype.Timestamptz `json:"created_at"`
	UpdatedAt       pgtype.Timestamptz `json:"updated_at"`
	Title           string             `json:"title"`
	FilePath        string             `json:"file_path"`
	FileName        string             `json:"file_name"`
	Container       string             `json:"container"`
	Size            int64              `json:"size"`
	ContentType     string             `json:"content_type"`
	RunTime         int32              `json:"run_time"`
	Adult           bool               `json:"adult"`
	TagLine         string             `json:"tag_line"`
	Summary         string             `json:"summary"`
	Art             string             `json:"art"`
	Thumb           string             `json:"thumb"`
	TmdbID          string             `json:"tmdb_id"`
	ImdbID          string             `json:"imdb_id"`
	Year            int32              `json:"year"`
	ReleaseDate     pgtype.Date        `json:"release_date"`
	Budget          int64              `json:"budget"`
	Revenue         int64              `json:"revenue"`
	ContentRating   string             `json:"content_rating"`
	AudienceRating  float32            `json:"audience_rating"`
	CriticRating    float32            `json:"critic_rating"`
	SpokenLanguages string             `json:"spoken_languages"`
	Genres          interface{}        `json:"genres"`
	Studios         interface{}        `json:"studios"`
	Cast            interface{}        `json:"cast"`
	Crew            interface{}        `json:"crew"`
	Extras          interface{}        `json:"extras"`
	VideoStreams    interface{}        `json:"video_streams"`
	AudioStreams    interface{}        `json:"audio_streams"`
	Subtitles       interface{}        `json:"subtitles"`
	Chapters        interface{}        `json:"chapters"`
}

func (q *Queries) GetMovieDetails(ctx context.Context, id int32) (GetMovieDetailsRow, error) {
	row := q.db.QueryRow(ctx, getMovieDetails, id)
	var i GetMovieDetailsRow
	err := row.Scan(
		&i.ID,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.Title,
		&i.FilePath,
		&i.FileName,
		&i.Container,
		&i.Size,
		&i.ContentType,
		&i.RunTime,
		&i.Adult,
		&i.TagLine,
		&i.Summary,
		&i.Art,
		&i.Thumb,
		&i.TmdbID,
		&i.ImdbID,
		&i.Year,
		&i.ReleaseDate,
		&i.Budget,
		&i.Revenue,
		&i.ContentRating,
		&i.AudienceRating,
		&i.CriticRating,
		&i.SpokenLanguages,
		&i.Genres,
		&i.Studios,
		&i.Cast,
		&i.Crew,
		&i.Extras,
		&i.VideoStreams,
		&i.AudioStreams,
		&i.Subtitles,
		&i.Chapters,
	)
	return i, err
}

const getMovieForDirectPlayback = `-- name: GetMovieForDirectPlayback :one
SELECT id, title, thumb, file_path FROM movies WHERE id = $1
`

type GetMovieForDirectPlaybackRow struct {
	ID       int32  `json:"id"`
	Title    string `json:"title"`
	Thumb    string `json:"thumb"`
	FilePath string `json:"file_path"`
}

func (q *Queries) GetMovieForDirectPlayback(ctx context.Context, id int32) (GetMovieForDirectPlaybackRow, error) {
	row := q.db.QueryRow(ctx, getMovieForDirectPlayback, id)
	var i GetMovieForDirectPlaybackRow
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Thumb,
		&i.FilePath,
	)
	return i, err
}

const getMovieForStreaming = `-- name: GetMovieForStreaming :one
SELECT id, file_path, content_type, size FROM movies
WHERE id = $1
`

type GetMovieForStreamingRow struct {
	ID          int32  `json:"id"`
	FilePath    string `json:"file_path"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func (q *Queries) GetMovieForStreaming(ctx context.Context, id int32) (GetMovieForStreamingRow, error) {
	row := q.db.QueryRow(ctx, getMovieForStreaming, id)
	var i GetMovieForStreamingRow
	err := row.Scan(
		&i.ID,
		&i.FilePath,
		&i.ContentType,
		&i.Size,
	)
	return i, err
}

const getMoviesPaginated = `-- name: GetMoviesPaginated :many
SELECT 
    id,
    title,
    thumb,
    year
FROM movies
ORDER BY title ASC
LIMIT $1 OFFSET $2
`

type GetMoviesPaginatedParams struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

type GetMoviesPaginatedRow struct {
	ID    int32  `json:"id"`
	Title string `json:"title"`
	Thumb string `json:"thumb"`
	Year  int32  `json:"year"`
}

func (q *Queries) GetMoviesPaginated(ctx context.Context, arg GetMoviesPaginatedParams) ([]GetMoviesPaginatedRow, error) {
	rows, err := q.db.Query(ctx, getMoviesPaginated, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []GetMoviesPaginatedRow{}
	for rows.Next() {
		var i GetMoviesPaginatedRow
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Thumb,
			&i.Year,
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
