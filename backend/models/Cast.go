package models

import "gorm.io/gorm"

type Cast struct {
	gorm.Model
	Character string `gorm:"not null" json:"character"`
	Order     uint   `json:"order"`
	MovieID   uint
	Movie     Movie
	ArtistID  uint
	Artist    Artist
}
