package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type crewHandlers struct {
	db *gorm.DB
}

func (h *crewHandlers) GetCrewMemberByName(c *fiber.Ctx) error {
	var member models.CrewMember
	name := c.Params("name")

	err := h.db.Where("name = ?", name).First(&member).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  member,
	})
}

func NewCrewHandlers(db *gorm.DB) *crewHandlers {
	return &crewHandlers{db: db}
}

func (h *crewHandlers) FindOrCreateCrewMember(c *fiber.Ctx) error {
	var member models.CrewMember

	err := c.BodyParser(&member)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.FirstOrCreate(&member).Error
	if err != nil {
		statusCode := getStatusCode(err)

		return c.Status(statusCode).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"error": false,
		"item":  member,
	})
}
