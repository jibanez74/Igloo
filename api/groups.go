package main

import (
	"igloo/handlers"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func musicMoodRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewMusicMoodHandlers(db)

	group := app.Group("/api/v1/music-mood")

	group.Get("/tag/:tag", h.GetMusicMoodByTag)
	group.Get("", h.GetMusicMoods)
	group.Post("", h.FindOrCreateMusicMood)
}

func musicGenreRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewMusicGenreHandlers(db)

	group := app.Group("/api/v1/music-genre")

	group.Get("/tag/:tag", h.GetMusicGenreByTag)
	group.Get("", h.GetMusicGenres)
	group.Post("", h.FindOrCreateMusicGenre)
}

func musicianRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewMusicianHandlers(db)

	group := app.Group("/api/v1/musician")

	group.Get("/:id", h.GetMusicianByID)
	group.Get("/name/:name", h.GetMusicianByName)
	group.Post("", h.CreateMusician)
}

func albumRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewAlbumHandlers(db)

	group := app.Group("/api/v1/album")

	group.Get("/:id", h.GetAlbumByID)
	group.Get("/title/:title", h.GetAlbumByTitle)
	group.Post("", h.CreateAlbum)
}

func trackRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewTrackHandlers(db)

	group := app.Group("/api/v1/track")

	group.Get("/:id", h.GetTrackByID)
	group.Post("", h.CreateTrack)
}

func movieRoutes(app *fiber.App, db *gorm.DB) {
	h := handlers.NewMovieHandlers(db)

	group := app.Group("/api/v1/movie")

	group.Get("/:id", h.GetMovieByID)
	group.Get("", h.GetMoviesWithPagination)
	group.Post("", h.CreateMovie)
	group.Put("", h.UpdateMovie)
}
