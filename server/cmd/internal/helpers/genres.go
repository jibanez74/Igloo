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
