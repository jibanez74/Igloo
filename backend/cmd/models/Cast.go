package models

import "gorm.io/gorm"

type Cast struct {
	gorm.Model
	ArtistID  uint   `json:"artistID"`
	Artist    Artist `json:"artist"`
	MovieID   uint   `json:"movieID"`
	Movie     Movie  `json:"movie"`
	Character string `gorm:"not null;default:'unknown'" json:"character"`
	Order     int64  `json:"order"`
}
