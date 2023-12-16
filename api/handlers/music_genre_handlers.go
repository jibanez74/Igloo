package handlers

import (
	"igloo/models"

	"gorm.io/gorm"
)

// create handler to find a music genre by its name, if not found then create it
// if found, then return the music genre
// if not found, then create it and return the music genre
func (h *appHandler) Find 