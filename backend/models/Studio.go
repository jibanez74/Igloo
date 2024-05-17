package models

import "gorm.io/gorm"

type Studio struct {
	gorm.Model
	Name    string   `gorm:"not null" json:"name"`
	Thumb   string   `gorm:"default:'no_thumb.png'" json:"thumb"`
	Country string   `json:"country"`
	Movies  []*Movie `gorm:"many2many:movie_studios" json:"movies"`
}
