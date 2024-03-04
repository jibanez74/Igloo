package routes

import (
	"igloo/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func StudioRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/studio")

	route.Get("/:id", func(c *fiber.Ctx) error {
		var studio models.Studio

		id := c.Params("id")

		studioId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Unable to parse studio id",
			})
		}

		if err = db.First(&studio, uint(studioId)).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   false,
				"message": err,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"error":  false,
			"studio": studio,
		})
	})

	route.Post("", func(c *fiber.Ctx) error {
		var studio models.Studio

		if err := c.BodyParser(&studio); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Where("title = ?", studio.Title).FirstOrCreate(&studio).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error":  false,
			"studio": studio,
		})
	})
}
