package repository

import (
	"net/http"

	"gorm.io/gorm"
)

func New(db *gorm.DB) Repo {
	return &repo{db: db}
}

func (r *repo) gormStatusCode(err error) int {
	switch err {
	case gorm.ErrInvalidValue:
		return http.StatusBadRequest
	case gorm.ErrInvalidData:
		return http.StatusBadRequest
	case gorm.ErrForeignKeyViolated:
		return http.StatusConflict
	case gorm.ErrInvalidTransaction:
		return http.StatusBadRequest
	case gorm.ErrDryRunModeUnsupported:
		return http.StatusNotImplemented
	case gorm.ErrRecordNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
