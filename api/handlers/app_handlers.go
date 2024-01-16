package handlers

import "gorm.io/gorm"

type appHandlers struct {
	db *gorm.DB
}

func NewAppHandlers(db *gorm.DB) *appHandlers {
	return &appHandlers{db}
}
