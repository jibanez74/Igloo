package handlers

import (
	"igloo/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type castHandlers struct {
	db *gorm.DB
}

func NewCastHandlers(db *gorm.DB) *castHandlers {
	return &castHandlers{db: db}
}

func (h *castHandlers) GetCastMemberByName(c *fiber.Ctx) error {
	var member models.CastMember
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

func (h *castHandlers) FindOrCreateCastMember(c *fiber.Ctx) error {
	var member models.CastMember

	err := c.BodyParser(&member)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": err,
		})
	}

	err = h.db.FirstOrCreate(&member, &member).Error
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
