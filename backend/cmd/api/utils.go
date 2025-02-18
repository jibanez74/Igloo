package main

import (
	"context"
	"errors"
	"fmt"
	"igloo/cmd/internal/database"

	"github.com/gofiber/fiber/v2"
)

type createArtistArgs struct {
	ID           int32
	Name         string
	OriginalName string
	Thumb        string
}

func (app *application) createOrGetArtist(c *fiber.Ctx, q *database.Queries, a *createArtistArgs) (int32, error) {
	if a == nil {
		return 0, errors.New("artist is nil")
	}

	existingArtist, err := q.GetArtistByTmdbID(c.Context(), a.ID)
	if err == nil {
		return existingArtist.ID, nil
	}

	thumbUrl := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", a.Thumb)

	artist := database.CreateArtistParams{
		Name:         a.Name,
		OriginalName: a.OriginalName,
		TmdbID:       a.ID,
		Thumb:        thumbUrl,
	}

	newArtist, err := q.CreateArtist(c.Context(), artist)
	if err != nil {
		return 0, fmt.Errorf("failed to create artist: %v", err)
	}

	if app.settings.GetDownloadImages() {
		app.imageQueue <- imageJob{
			sourceURL:  thumbUrl,
			targetPath: app.settings.GetArtistsImgDir(),
			filename:   fmt.Sprintf("%d.jpg", newArtist.ID),
			onSuccess: func(localPath string) {
				if localPath != thumbUrl {
					_, err := q.UpdateArtistThumb(context.Background(), database.UpdateArtistThumbParams{
						ID:    newArtist.ID,
						Thumb: localPath,
					})
					if err != nil {
						app.logger.Error(fmt.Errorf("failed to update artist thumb: %w", err))
					}
				}
			},
			onError: func(err error) {
				app.logger.Error(fmt.Errorf("failed to save artist image: %w", err))
			},
		}
	}

	return newArtist.ID, nil
}
