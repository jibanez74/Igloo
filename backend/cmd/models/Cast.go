package models

import "gorm.io/gorm"

type Cast struct {
	gorm.Model
	Character string `gorm:"not null;default:'unknown'"`
	Order     uint
	MovieID   uint
	Movie     Movie
	ArtistID  uint
	Artist    Artist
}
