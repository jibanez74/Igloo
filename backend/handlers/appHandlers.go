package handlers

import "gorm.io/gorm"

type AppHandlers struct {
	db *gorm.DB
}

func New(db *gorm.DB) *AppHandlers {
	return &AppHandlers{
		db: db,
	}
}
