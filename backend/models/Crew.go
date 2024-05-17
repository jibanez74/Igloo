package models

import "gorm.io/gorm"

type Crew struct {
	gorm.Model
	Job        string `gorm:"not null" json:"job"`
	Department string `gorm:"not null" json:"department"`
	MovieID    uint
	Movie      Movie
	ArtistID   uint
	Artist     Artist
}
