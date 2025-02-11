package main

import (
	"errors"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

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

	var artist database.CreateArtistParams
	artistUrl := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", a.Thumb)

	if app.settings.GetDownloadImages() {
		fullPath, err := helpers.SaveImage(
			artistUrl,
			app.settings.GetArtistsImgDir(),
			fmt.Sprintf("%d.jpg", a.ID),
		)

		if err == nil {
			artist.Thumb = *fullPath
		}
	} else {
		artist.Thumb = artistUrl
	}

	artist.Name = a.Name
	artist.OriginalName = a.OriginalName
	artist.TmdbID = a.ID

	newArtist, err := q.CreateArtist(c.Context(), artist)
	if err != nil {
		return 0, fmt.Errorf("failed to create artist: %v", err)
	}

	return newArtist.ID, nil
}
