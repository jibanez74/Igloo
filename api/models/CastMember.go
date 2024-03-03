package models

import "gorm.io/gorm"

type CastMember struct {
	gorm.Model
	Artist    Artist
	ArtistID  uint
	Movie     Movie
	MovieID   uint
	Character string `gorm:"size:80;default:'unknown'" json:"character"`
	Order     uint   `json:"order"`
}
