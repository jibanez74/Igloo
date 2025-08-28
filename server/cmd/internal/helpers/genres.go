package helpers

import (
	"context"
	"errors"
	"igloo/cmd/internal/database"
	"strings"
)

type SaveGenresParams struct {
	Tag        string
	GenreType  string
	MusicianID int32
	AlbumID    int32
	TrackID    int32
	MovieID    int32
}

func SaveGenres(ctx context.Context, qtx *database.Queries, data *SaveGenresParams) error {
	if data == nil {
		return errors.New("got nil value for data in SaveGenres function")
	}

	if qtx == nil {
		return errors.New("got nil value for qtx in SaveGenres function")
	}

	if data.Tag == "" {
		return errors.New("got nil value for data in SaveGenres function")
	}

	if data.GenreType == "" {
		return errors.New("got empty string for data.GenreType in SaveGenres function")
	}

	tag := strings.ToLower(strings.TrimSpace(data.Tag))

	exist, err := qtx.CheckGenreExistByTag(ctx, tag)
	if err != nil {
		return err
	}

	var genre database.Genre

	if exist {
		genre, err = qtx.GetGenreByTag(ctx, tag)
		if err != nil {
			return err
		}
	} else {
		genre, err = qtx.CreateGenre(ctx, database.CreateGenreParams{
			Tag:       tag,
			GenreType: data.GenreType,
		})

		if err != nil {
			return err
		}
	}

	if genre.GenreType == "music" {
		err = qtx.CreateMusicianGenre(ctx, database.CreateMusicianGenreParams{
			MusicianID: data.MusicianID,
			GenreID:    genre.ID,
		})

		if err != nil {
			return err
		}

		err = qtx.CreateAlbumGenre(ctx, database.CreateAlbumGenreParams{
			AlbumID: data.AlbumID,
			GenreID: genre.ID,
		})

		if err != nil {
			return err
		}

		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
			TrackID: data.TrackID,
			GenreID: genre.ID,
		})

		if err != nil {
			return err
		}
	}

	if genre.GenreType == "movie" {
		err = qtx.AddMovieGenre(ctx, database.AddMovieGenreParams{
			MovieID: data.MovieID,
			GenreID: genre.ID,
		})

		if err != nil {
			return err
		}
	}

	return nil
}
