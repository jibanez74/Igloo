package main

import (
	"igloo/database"
	"igloo/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const PORT = ":8080"

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic("Unable to connect to db")
	}

	app := fiber.New()
	app.Use(recover.New())

	// music routes
	routes.MusicGenreRoutes(app, db)
	routes.MusicMoodRoutes(app, db)
	routes.MusicianRoutes(app, db)
	routes.AlbumRoutes(app, db)
	routes.TrackRoutes(app, db)

	// movie routes
	routes.ArtistRoutes(app, db)
	routes.MovieGenreRoutes(app, db)
	routes.StudioRoutes(app, db)
	routes.MovieRoutes(app, db)
	routes.CastRoutes(app, db)
	routes.CrewRoutes(app, db)

	app.Listen(PORT)
}
