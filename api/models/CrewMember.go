package models

import "gorm.io/gorm"

type CrewMember struct {
	gorm.Model
	ArtistID   uint
	Artist     Artist
	Movie      Movie
	MovieID    uint
	Department string `json:"department"`
	Job        string `json:"job"`
}
