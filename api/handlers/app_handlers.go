package handlers

import (
	"gorm.io/gorm"
)

type appHandler struct {
	db *gorm.DB
}

func NewAppHandler(db *gorm.DB) *appHandler {
	return &appHandler{
		db: db,
	}
}