package models

import "gorm.io/gorm"

type Subtitles struct {
	gorm.Model
	Language string `gorm:"not null;default:'unknown'"`
	Codec    string `gorm:"not null;default:'unknown'"`
	Index    uint
	MovieID  uint
	Movie    Movie
}
