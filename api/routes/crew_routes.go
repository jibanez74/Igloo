package routes

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func CrewRoutes(app *fiber.App, db *gorm.DB) {
	route := app.Group("/api/v1/crew")

	route.Post("", func(c *fiber.Ctx) error {
		var crew models.CrewMember

		if err := c.BodyParser(&crew); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		if err := db.Create(&crew).Error; err != nil {
			return c.Status(getStatusCode(err)).JSON(fiber.Map{
				"error":   true,
				"message": err,
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"error": false,
			"crew":  crew,
		})
	})
}
