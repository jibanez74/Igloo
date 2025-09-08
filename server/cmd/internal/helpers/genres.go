package helpers

import (
	"context"
	"fmt"
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
		// TrackID is required for music genres
		if data.TrackID <= 0 {
			return fmt.Errorf("TrackID is required for music genres")
		}

		// Only create musician-genre relationship if MusicianID is provided (non-zero)
		if data.MusicianID > 0 {
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
		}

		// Only create album-genre relationship if AlbumID is provided (non-zero)
		if data.AlbumID > 0 {
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
		// MovieID is required for movie genres
		if data.MovieID <= 0 {
			return fmt.Errorf("MovieID is required for movie genres")
		}

		// Only create movie-genre relationship if MovieID is provided (non-zero)
		if data.MovieID > 0 {
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
