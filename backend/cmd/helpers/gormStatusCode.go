package helpers

import (
	"net/http"

	"gorm.io/gorm"
)

func GormStatusCode(err error) int {
	switch err {
	case gorm.ErrRecordNotFound:
		return http.StatusNotFound
	case gorm.ErrDuplicatedKey:
		return http.StatusConflict
	case gorm.ErrInvalidData:
		return http.StatusBadRequest
	case gorm.ErrInvalidTransaction:
		return http.StatusInternalServerError
	case gorm.ErrUnsupportedRelation:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}
