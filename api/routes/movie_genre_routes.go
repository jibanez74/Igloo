package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MovieGenreRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/movie-genre")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var movieGenre models.MovieGenre

		id := c.Params("id")

		movieGenreId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse movie genre id",
			})
		}

		if err = db.First(&movieGenre, uint(movieGenreId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error": false,
			"genre": movieGenre,
		})
	})

	route.Post("", func(c *fiber.Ctx) error {
		var movieGenre models.MovieGenre

		if err := c.BodyParser(&movieGenre); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("tag = ?", movieGenre.Tag).FirstOrCreate(&movieGenre).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"genre": movieGenre,
		})
	})
}
