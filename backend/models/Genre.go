package models

import "gorm.io/gorm"

type Genre struct {
	gorm.Model
	Tag       string   `gorm:"not null;index"`
	GenreType string   `gorm:"not null" json:"genreType"`
	Movies    []*Movie `gorm:"many2many:movie_genres"`
}
