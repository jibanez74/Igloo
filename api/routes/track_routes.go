package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func TrackRoutes(app *fiber.App, db *gorm.DB) {
	routes := app.Group("/api/v1/track")

	routes.Get("/:id", func(c *fiber.Ctx) error {
		var track models.Track

		id := c.Params("id")

		trackId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse track id",
			})
		}

		if err = db.First(&track, uint(trackId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error": false,
			"track": track,
		})
	})

	routes.Post("", func(c *fiber.Ctx) error {
		var track models.Track

		if err := c.BodyParser(&track); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("title = ?", track.Title).FirstOrCreate(&track).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"track": track,
		})
	})
}
