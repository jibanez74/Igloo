package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MusicGenreRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/music-genre")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var genre models.MusicGenre

		id := c.Params("id")

		genreId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse genre id",
			})
		}

		if err = db.First(&genre, uint(genreId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error": false,
			"genre": genre,
		})
	})

	route.Get("", func(c *fiber.Ctx) error {
		var genres []models.MusicGenre

		if err := db.Find(&genres).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":  false,
			"genres": genres,
		})
	})

	route.Post("", func(c *fiber.Ctx) error {
		var genre models.MusicGenre

		if err := c.BodyParser(&genre); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("tag = ?", genre.Tag).FirstOrCreate(&genre).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"genre": genre,
		})
	})
}
