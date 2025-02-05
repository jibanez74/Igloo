// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type Querier interface {
	AddMovieGenre(ctx context.Context, arg AddMovieGenreParams) error
	AddMovieStudio(ctx context.Context, arg AddMovieStudioParams) error
	AddUserMovie(ctx context.Context, arg AddUserMovieParams) error
	CreateArtist(ctx context.Context, arg CreateArtistParams) (Artist, error)
	CreateAudioStream(ctx context.Context, arg CreateAudioStreamParams) (AudioStream, error)
	CreateCastMember(ctx context.Context, arg CreateCastMemberParams) (CastList, error)
	CreateChapter(ctx context.Context, arg CreateChapterParams) (Chapter, error)
	CreateCrewMember(ctx context.Context, arg CreateCrewMemberParams) (CrewList, error)
	CreateGenre(ctx context.Context, arg CreateGenreParams) (Genre, error)
	CreateGlobalSettings(ctx context.Context, arg CreateGlobalSettingsParams) (GlobalSetting, error)
	CreateMovie(ctx context.Context, arg CreateMovieParams) (Movie, error)
	CreateMovieExtra(ctx context.Context, arg CreateMovieExtraParams) (MovieExtra, error)
	CreateStudio(ctx context.Context, arg CreateStudioParams) (Studio, error)
	CreateSubtitle(ctx context.Context, arg CreateSubtitleParams) (Subtitle, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	CreateVideoStream(ctx context.Context, arg CreateVideoStreamParams) (VideoStream, error)
	GetActiveUserByEmailAndUsername(ctx context.Context, arg GetActiveUserByEmailAndUsernameParams) (User, error)
	GetActiveUserByEmailOrUsername(ctx context.Context, email string) (User, error)
	GetGenreByID(ctx context.Context, id int32) (string, error)
	GetGenreByTmdbID(ctx context.Context, tmdbID int32) (GetGenreByTmdbIDRow, error)
	GetGlobalSettings(ctx context.Context) (GlobalSetting, error)
	GetLatestMovies(ctx context.Context) ([]GetLatestMoviesRow, error)
	GetMovie(ctx context.Context, id int32) (Movie, error)
	GetMovieByTitle(ctx context.Context, title string) (Movie, error)
	GetMovieByTmdbID(ctx context.Context, tmdbID string) (string, error)
	GetMovieGenres(ctx context.Context, movieID int32) ([]Genre, error)
	GetMovieStudios(ctx context.Context, movieID int32) ([]Studio, error)
	GetMovieSubtitles(ctx context.Context, movieID pgtype.Int4) ([]Subtitle, error)
	GetMovieVideoStreams(ctx context.Context, movieID pgtype.Int4) ([]VideoStream, error)
	GetMoviesAlphabetically(ctx context.Context) ([]GetMoviesAlphabeticallyRow, error)
	GetStudioByTmdbID(ctx context.Context, tmdbID int32) (int32, error)
	GetUser(ctx context.Context, id int32) (User, error)
	GetUserMovies(ctx context.Context, userID int32) ([]Movie, error)
	RemoveMovieGenre(ctx context.Context, arg RemoveMovieGenreParams) error
	RemoveMovieStudio(ctx context.Context, arg RemoveMovieStudioParams) error
	RemoveUserMovie(ctx context.Context, arg RemoveUserMovieParams) error
	UpdateGlobalSettings(ctx context.Context, arg UpdateGlobalSettingsParams) (GlobalSetting, error)
}

var _ Querier = (*Queries)(nil)
