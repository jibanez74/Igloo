package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func CastRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/cast")

	route.Post("", func(c *fiber.Ctx) error {
		var cast models.CastMember

		if err := c.BodyParser(&cast); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Create(&cast).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"cast":  cast,
		})
	})
}
