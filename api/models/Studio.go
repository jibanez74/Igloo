package models

import "gorm.io/gorm"

type Studio struct {
	gorm.Model
	Title   string   `gorm:"size:80;not null;index" json:"title"`
	Thumb   string   `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Country string   `gorm:"size:40;default:'unknown'" json:"country"`
	Movies  []*Movie `gorm:"many2many:movie_studios" json:"movies"`
}
