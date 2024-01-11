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
	musicGenreRepo := repository.NewMusicGenreRepo(db)
	musicGenreHandler := handlers.NewMusicGenreHandler(musicGenreRepo)

	app := fiber.New()
	app.Use(recover.New())

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
