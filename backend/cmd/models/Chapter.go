package models

import "gorm.io/gorm"

type Chapter struct {
	gorm.Model
	Title   string `gorm:"not null;default:'unknown'" json:"title"`
	Start   uint   `json:"start"`
	Thumb   string `gorm:"not null;default:'unknown'" json:"thumb"`
	MovieID uint   `json:"movieID"`
	Movie   Movie  `json:"movie"`
}
