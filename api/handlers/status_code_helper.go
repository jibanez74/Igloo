package handlers

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func getStatusCode(err error) int {
	switch err {
	case gorm.ErrRecordNotFound:
		return fiber.StatusNotFound
	case gorm.ErrDuplicatedKey:
		return fiber.StatusConflict
	case gorm.ErrInvalidData:
		return fiber.StatusBadRequest
	case gorm.ErrInvalidTransaction:
		return fiber.StatusInternalServerError
	case gorm.ErrUnsupportedRelation:
		return fiber.StatusUnprocessableEntity
	default:
		return fiber.StatusInternalServerError
	}
}
