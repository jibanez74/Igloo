package main

import (
	"igloo/database"
	"igloo/handlers"
	"igloo/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"gorm.io/gorm"
)

func setupApp(db *gorm.DB) *fiber.App {
	musicianRepo := repository.NewMusicianRepo(db)
	musicianHandler := handlers.NewMusicianHandler(musicianRepo)

	albumRepo := repository.NewAlbumRepo(db)
	albumHandler := handlers.NewAlbumHandler(albumRepo)

	musicGenreRepo := repository.NewMusicGenreRepo(db)
	musicGenreHandler := handlers.NewMusicGenreHandler(musicGenreRepo)

	// configure a new fiber app
	app := fiber.New()
	app.Use(recover.New())

	// musician routes
	app.Get("/api/v1/musician", musicianHandler.GetMusicians)
	app.Post("/api/v1/musician", musicianHandler.CreateMusician)

	// album routes
	app.Get("/api/v1/album/title/:title", albumHandler.GetAlbumByTitle)
	app.Get("/api/v1/album/id/:id", albumHandler.GetAlbumByID)
	app.Get("/api/v1/album", albumHandler.GetAlbums)
	app.Post("/api/v1/album", albumHandler.CreateAlbum)

	// music genres routes
	app.Get("/api/v1/music-genre", musicGenreHandler.GetMusicGenres)

	return app
}

func main() {
	// init connection to database
	db, err := database.InitDatabase()
	if err != nil {
		panic(err)
	}

	// setup and start the Fiber app
	app := setupApp(db)
	app.Listen(":8080")
}
