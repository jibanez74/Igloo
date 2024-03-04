package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ArtistRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/artist")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var artist models.Artist

		id := c.Params("id")

		artistId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse artist id",
			})
		}

		if err = db.First(&artist, uint(artistId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":  false,
			"artist": artist,
		})
	})

	route.Post("", func(c *fiber.Ctx) error {
		var artist models.Artist

		if err := c.BodyParser(&artist); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("name = ?", artist.Name).FirstOrCreate(&artist).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error":  false,
			"artist": artist,
		})
	})
}
