// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: user_movies.sql

package database

import (
	"context"
)

const addUserMovie = `-- name: AddUserMovie :exec
INSERT INTO user_movies (user_id, movie_id)
VALUES ($1, $2)
`

type AddUserMovieParams struct {
	UserID  int32 `json:"user_id"`
	MovieID int32 `json:"movie_id"`
}

func (q *Queries) AddUserMovie(ctx context.Context, arg AddUserMovieParams) error {
	_, err := q.db.Exec(ctx, addUserMovie, arg.UserID, arg.MovieID)
	return err
}

const getUserLikedMovies = `-- name: GetUserLikedMovies :many
SELECT m.id, m.created_at, m.updated_at, m.title, m.file_path, m.file_name, m.container, m.size, m.content_type, m.run_time, m.adult, m.tag_line, m.summary, m.art, m.thumb, m.tmdb_id, m.imdb_id, m.year, m.release_date, m.budget, m.revenue, m.content_rating, m.audience_rating, m.critic_rating, m.spoken_languages FROM movies m
JOIN user_movies um ON um.movie_id = m.id
WHERE um.user_id = $1 AND um.liked = TRUE
`

func (q *Queries) GetUserLikedMovies(ctx context.Context, userID int32) ([]Movie, error) {
	rows, err := q.db.Query(ctx, getUserLikedMovies, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Movie{}
	for rows.Next() {
		var i Movie
		if err := rows.Scan(
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

const getUserMovies = `-- name: GetUserMovies :many
SELECT m.id, m.created_at, m.updated_at, m.title, m.file_path, m.file_name, m.container, m.size, m.content_type, m.run_time, m.adult, m.tag_line, m.summary, m.art, m.thumb, m.tmdb_id, m.imdb_id, m.year, m.release_date, m.budget, m.revenue, m.content_rating, m.audience_rating, m.critic_rating, m.spoken_languages FROM movies m
JOIN user_movies um ON um.movie_id = m.id
WHERE um.user_id = $1
`

func (q *Queries) GetUserMovies(ctx context.Context, userID int32) ([]Movie, error) {
	rows, err := q.db.Query(ctx, getUserMovies, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Movie{}
	for rows.Next() {
		var i Movie
		if err := rows.Scan(
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

const likeMovie = `-- name: LikeMovie :exec
UPDATE user_movies
SET liked = TRUE
WHERE user_id = $1 AND movie_id = $2
`

type LikeMovieParams struct {
	UserID  int32 `json:"user_id"`
	MovieID int32 `json:"movie_id"`
}

func (q *Queries) LikeMovie(ctx context.Context, arg LikeMovieParams) error {
	_, err := q.db.Exec(ctx, likeMovie, arg.UserID, arg.MovieID)
	return err
}

const removeUserMovie = `-- name: RemoveUserMovie :exec
DELETE FROM user_movies
WHERE user_id = $1 AND movie_id = $2
`

type RemoveUserMovieParams struct {
	UserID  int32 `json:"user_id"`
	MovieID int32 `json:"movie_id"`
}

func (q *Queries) RemoveUserMovie(ctx context.Context, arg RemoveUserMovieParams) error {
	_, err := q.db.Exec(ctx, removeUserMovie, arg.UserID, arg.MovieID)
	return err
}

const unlikeMovie = `-- name: UnlikeMovie :exec
UPDATE user_movies
SET liked = FALSE
WHERE user_id = $1 AND movie_id = $2
`

type UnlikeMovieParams struct {
	UserID  int32 `json:"user_id"`
	MovieID int32 `json:"movie_id"`
}

func (q *Queries) UnlikeMovie(ctx context.Context, arg UnlikeMovieParams) error {
	_, err := q.db.Exec(ctx, unlikeMovie, arg.UserID, arg.MovieID)
	return err
}
