package models

import "gorm.io/gorm"

type Subtitles struct {
	gorm.Model
	Title     string `gorm:"not null;default:'unknown'"`
	Language  string `gorm:"not null;default:'unknown'"`
	Codec     string `gorm:"not null;default:'unknown'"`
	IsDefault bool   `gorm:"default:false"`
	IsForced  bool   `gorm:"default:false"`
	MovieID   uint
	Movie     Movie
}
