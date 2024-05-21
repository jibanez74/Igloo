package models

import "gorm.io/gorm"

type Trailer struct {
	gorm.Model
	Title   string `gorm:"not nul;default:'unknown'"`
	Url     string `gorm:"not null"`
	MovieID uint
	Movie   Movie
}
