package main

import (
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

	return newArtist.ID, nil
}
