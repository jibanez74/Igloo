package models

import "gorm.io/gorm"

type Studio struct {
	gorm.Model
	Name    string   `gorm:"not null;index"`
	Thumb   string   `gorm:"not null;default:'no_thumb.png'"`
	Country string   `gorm:"not null;default:'unknown'"`
	Movies  []*Movie `gorm:"many2many:movie_studios" json:"movies"`
}
