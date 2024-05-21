package models

import "gorm.io/gorm"

type Chapter struct {
	gorm.Model
	Title   string `gorm:"not null;default:'unknown'"`
	Start   uint
	Thumb   string `gorm:"not null;default:'no_chapter_thumb.png'"`
	MovieID uint
	Movie   Movie
}
