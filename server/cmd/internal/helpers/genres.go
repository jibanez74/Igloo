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
		exist, err = qtx.CheckMusicianGenreExist(ctx, database.CheckMusicianGenreExistParams{
			MusicianID: data.MusicianID,
			GenreID:    genre.ID,
		})

		if err != nil {
			return err
		}

		if !exist {
			err = qtx.CreateMusicianGenre(ctx, database.CreateMusicianGenreParams{
				MusicianID: data.MusicianID,
				GenreID:    genre.ID,
			})

			if err != nil {
				return err
			}
		}

		exist, err = qtx.CheckAlbumGenreExist(ctx, database.CheckAlbumGenreExistParams{
			AlbumID: data.AlbumID,
			GenreID: genre.ID,
		})

		if err != nil {
			return err
		}

		if !exist {
			err = qtx.CreateAlbumGenre(ctx, database.CreateAlbumGenreParams{
				AlbumID: data.AlbumID,
				GenreID: genre.ID,
			})

			if err != nil {
				return err
			}
		}

		exist, err = qtx.CheckTrackGenreExists(ctx, database.CheckTrackGenreExistsParams{
			TrackID: data.TrackID,
			GenreID: genre.ID,
		})

		if err != nil {
			return err
		}

		if !exist {
			err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
				TrackID: data.TrackID,
				GenreID: genre.ID,
			})

			if err != nil {
				return err
			}

		}

	}

	if genre.GenreType == "movie" {
		exist, err = qtx.CheckMovieGenreExists(ctx, database.CheckMovieGenreExistsParams{
			MovieID: data.MovieID,
			GenreID: genre.ID,
		})

		if err != nil {
			return err
		}

		if !exist {
			err = qtx.CreateMovieGenre(ctx, database.CreateMovieGenreParams{
				MovieID: data.MovieID,
				GenreID: genre.ID,
			})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func ParseGenres(genreString string) []string {
  if genreString == "" {
    return nil
  }

  if !strings.Contains(genreString, ",") {
    return []string{strings.TrimSpace(genreString)}
  }

  parts := strings.Split(genreString, ",")
  genres := make([]string, 0, len(parts))

  for _, part := range parts {
    if trimmed := strings.TrimSpace(part); trimmed != "" {
      genres = append(genres, trimmed)
    }
  }

  return genres
}
