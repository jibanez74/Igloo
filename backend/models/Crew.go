package models

import "gorm.io/gorm"

type Crew struct {
	gorm.Model
	Job        string `gorm:"not null;default:'unknown'"`
	Department string `gorm:"not null;default:'unknown'"`
	MovieID    uint
	Movie      Movie
	ArtistID   uint
	Artist     Artist
}
