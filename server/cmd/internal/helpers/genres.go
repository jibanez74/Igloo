package helpers

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"strings"

	"github.com/jackc/pgx/v5"
)

func SaveGenres(ctx context.Context, qtx *database.Queries, genreString, genreType string) ([]int32, error) {
	if genreString == "" {
		return nil, errors.New("got empty string for genre string in SaveGenres function")
	}

	var genreIDs []int32

	genres := strings.Split(genreString, ",")

	for _, genre := range genres {
		genre = strings.TrimSpace(genre)

		if genre == "" {
			continue
		}

		existingGenre, err := qtx.GetGenreByTagAndType(ctx, database.GetGenreByTagAndTypeParams{
			Tag:       genre,
			GenreType: genreType,
		})

		var genreID int32

		if err != nil {
			if err == pgx.ErrNoRows {
				newGenre, err := qtx.CreateGenre(ctx, database.CreateGenreParams{
					Tag:       genre,
					GenreType: genreType,
				})

				if err != nil {
					return nil, fmt.Errorf("failed to create genre %s: %w", genre, err)
				}

				genreID = newGenre.ID
			} else {

				return nil, fmt.Errorf("failed to check if genre %s exists: %w", genre, err)
			}
		} else {
			genreID = existingGenre.ID
		}

		genreIDs = append(genreIDs, genreID)
	}

	return genreIDs, nil
}

func SaveGenreMusic(ctx context.Context, qtx *database.Queries, track *database.Track, genreID int32) error {
	if track == nil {
		return errors.New("got nil pointer for track in SaveGenreMusic")
	}

	exist, err := qtx.CheckTrackGenreExists(ctx, database.CheckTrackGenreExistsParams{TrackID: track.ID, GenreID: genreID})
	if err != nil {
		return err
	}

	if !exist {
		err = qtx.CreateTrackGenre(ctx, database.CreateTrackGenreParams{TrackID: track.ID, GenreID: genreID})
		if err != nil {
			return err
		}
	}

	if track.MusicianID.Int32 > 0 {
		exist, err = qtx.CheckMusicianGenreExist(ctx, database.CheckMusicianGenreExistParams{MusicianID: track.MusicianID.Int32, GenreID: genreID})
		if err != nil {
			return err
		}

		if !exist {
			err = qtx.CreateMusicianGenre(ctx, database.CreateMusicianGenreParams{MusicianID: track.MusicianID.Int32, GenreID: genreID})
			if err != nil {
				return err
			}
		}
	}

	if track.AlbumID.Int32 > 0 {
		exist, err = qtx.CheckAlbumGenreExist(ctx, database.CheckAlbumGenreExistParams{AlbumID: track.AlbumID.Int32, GenreID: genreID})
		if err != nil {
			return err
		}

		if !exist {
			err = qtx.CreateAlbumGenre(ctx, database.CreateAlbumGenreParams{AlbumID: track.AlbumID.Int32, GenreID: genreID})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
