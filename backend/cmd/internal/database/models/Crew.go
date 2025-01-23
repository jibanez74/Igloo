package models

import "gorm.io/gorm"

type Crew struct {
	gorm.Model
	ArtistID   uint   `json:"artistID"`
	Artist     Artist `json:"artist"`
	MovieID    uint   `json:"movieID"`
	Movie      Movie  `json:"movie"`
	Job        string `gorm:"not null;default:'unknown'" json:"job"`
	Department string `gorm:"not null;default:'unknown'" json:"department"`
}
