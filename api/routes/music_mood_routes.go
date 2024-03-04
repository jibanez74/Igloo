package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func MusicMoodRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/music-mood")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var mood models.MusicMood

		id := c.Params("id")

		moodId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse mood id",
			})
		}

		if err = db.First(&mood, uint(moodId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error": false,
			"mood":  mood,
		})
	})

	route.Post("", func(c *fiber.Ctx) error {
		var mood models.MusicMood

		if err := c.BodyParser(&mood); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("tag = ?", mood.Tag).FirstOrCreate(&mood).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"mood":  mood,
		})
	})
}
