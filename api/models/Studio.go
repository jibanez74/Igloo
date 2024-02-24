package models

import "gorm.io/gorm"

type Studio struct {
	gorm.Model
	Title  string   `gorm:"size:40;not null;index" json:"title"`
	Thumb  string   `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Movies []*Movie `gorm:"many2many:movie_studios" json:"movies"`
}
