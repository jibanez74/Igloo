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
	CheckUserExists(ctx context.Context, arg CheckUserExistsParams) (bool, error)
	CleanupExpiredDeviceCodes(ctx context.Context) error
	CreateAudioStream(ctx context.Context, arg CreateAudioStreamParams) (AudioStream, error)
	CreateCastMember(ctx context.Context, arg CreateCastMemberParams) (CastList, error)
	CreateChapter(ctx context.Context, arg CreateChapterParams) (Chapter, error)
	CreateCrewMember(ctx context.Context, arg CreateCrewMemberParams) (CrewList, error)
	CreateDeviceCode(ctx context.Context, arg CreateDeviceCodeParams) (DeviceCode, error)
	CreateMovie(ctx context.Context, arg CreateMovieParams) (Movie, error)
	CreateMovieExtra(ctx context.Context, arg CreateMovieExtraParams) (MovieExtra, error)
	CreateSettings(ctx context.Context, arg CreateSettingsParams) (GlobalSetting, error)
	CreateSubtitle(ctx context.Context, arg CreateSubtitleParams) (Subtitle, error)
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	CreateVideoStream(ctx context.Context, arg CreateVideoStreamParams) (VideoStream, error)
	DeleteAllSettings(ctx context.Context) error
	DeleteUser(ctx context.Context, id int32) error
	GetDeviceCode(ctx context.Context, deviceCode string) (DeviceCode, error)
	GetDeviceCodeByUserCode(ctx context.Context, userCode string) (DeviceCode, error)
	GetGenreByID(ctx context.Context, id int32) (string, error)
	GetLatestMovies(ctx context.Context) ([]GetLatestMoviesRow, error)
	GetMovieByTmdbID(ctx context.Context, tmdbID string) (GetMovieByTmdbIDRow, error)
	GetMovieCount(ctx context.Context) (int64, error)
	GetMovieDetails(ctx context.Context, id int32) (GetMovieDetailsRow, error)
	GetMovieForDirectPlayback(ctx context.Context, id int32) (GetMovieForDirectPlaybackRow, error)
	GetMovieForStreaming(ctx context.Context, id int32) (GetMovieForStreamingRow, error)
	GetMovieStudios(ctx context.Context, movieID int32) ([]Studio, error)
	GetMovieVideoStreams(ctx context.Context, movieID pgtype.Int4) ([]VideoStream, error)
	GetMoviesPaginated(ctx context.Context, arg GetMoviesPaginatedParams) ([]GetMoviesPaginatedRow, error)
	GetOrCreateArtist(ctx context.Context, arg GetOrCreateArtistParams) (GetOrCreateArtistRow, error)
	GetOrCreateGenre(ctx context.Context, arg GetOrCreateGenreParams) (GetOrCreateGenreRow, error)
	GetOrCreateStudio(ctx context.Context, arg GetOrCreateStudioParams) (GetOrCreateStudioRow, error)
	GetSettings(ctx context.Context) (GlobalSetting, error)
	GetSettingsCount(ctx context.Context) (int64, error)
	GetTotalUsersCount(ctx context.Context) (int64, error)
	GetUserByID(ctx context.Context, id int32) (GetUserByIDRow, error)
	GetUserForLogin(ctx context.Context, arg GetUserForLoginParams) (User, error)
	GetUserLikedMovies(ctx context.Context, userID int32) ([]Movie, error)
	GetUserMovies(ctx context.Context, userID int32) ([]Movie, error)
	GetUsersPaginated(ctx context.Context, arg GetUsersPaginatedParams) ([]GetUsersPaginatedRow, error)
	LikeMovie(ctx context.Context, arg LikeMovieParams) error
	RemoveUserMovie(ctx context.Context, arg RemoveUserMovieParams) error
	UnlikeMovie(ctx context.Context, arg UnlikeMovieParams) error
	UpdateSettings(ctx context.Context, arg UpdateSettingsParams) (GlobalSetting, error)
	UpdateUser(ctx context.Context, arg UpdateUserParams) (UpdateUserRow, error)
	UpdateUserAvatar(ctx context.Context, arg UpdateUserAvatarParams) (UpdateUserAvatarRow, error)
	VerifyDeviceCode(ctx context.Context, arg VerifyDeviceCodeParams) error
}

var _ Querier = (*Queries)(nil)
