package repository

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func New(db *gorm.DB) Repo {
	return &repo{db: db}
}

func (r *repo) gormStatusCode(err error) int {
	switch err {
	case gorm.ErrInvalidValue:
		return fiber.StatusBadRequest
	case gorm.ErrInvalidData:
		return fiber.StatusBadRequest
	case gorm.ErrForeignKeyViolated:
		return fiber.StatusConflict
	case gorm.ErrInvalidTransaction:
		return fiber.StatusBadRequest
	case gorm.ErrDryRunModeUnsupported:
		return fiber.StatusNotImplemented
	case gorm.ErrRecordNotFound:
		return fiber.StatusNotFound
	default:
		return fiber.StatusInternalServerError
	}
}
