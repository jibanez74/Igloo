package main

import (
	"igloo/database"
	"igloo/handlers"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const PORT = ":8080"

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic("Unable to connect to db")
	}

	appHandlers := handlers.NewAppHandlers(db)

	app := fiber.New()
	app.Use(recover.New())
	app.Use(cors.New())

	// music genre routes
	app.Get("/api/v1/music-genre/tag/:tag", appHandlers.GetMusicGenreByTag)
	app.Get("/api/v1/music-genre", appHandlers.GetMusicGenres)
	app.Post("/api/v1/music-genre", appHandlers.FindOrCreateMusicGenre)

	// music mood routes
	app.Get("/api/v1/music-mood/tag/:tag", appHandlers.GetMusicMoodByTag)
	app.Get("/api/v1/music-mood", appHandlers.GetMusicMoods)
	app.Post("/api/v1/music-mood", appHandlers.FindOrCreateMusicMood)

	// musician routes
	app.Get("/api/v1/musician/name/:name", appHandlers.GetMusicianByName)
	app.Get("/api/v1/musician/id/:id", appHandlers.GetMusicianByID)
	app.Post("/api/v1/musician", appHandlers.CreateMusician)

	// album routes
	app.Get("/api/v1/album/title/:title", appHandlers.GetAlbumByTitle)
	app.Get("/api/v1/album/id/:id", appHandlers.GetAlbumByID)
	app.Get("/api/v1/album", appHandlers.GetAlbums)
	app.Post("/api/v1/album", appHandlers.CreateAlbum)

	// track routes
	app.Get("/api/v1/track/id/:id", appHandlers.GetTrackByID)
	app.Get("/api/v1/track", appHandlers.GetTracks)
	app.Post("/api/v1/track", appHandlers.CreateTrack)

	app.Listen(PORT)
}
