package models

import "gorm.io/gorm"

type MovieGenre struct {
	gorm.Model
	Tag    string   `gorm:"size:20;not null;index" json:"tag"`
	Movies []*Movie `gorm:"many2many:movie_genres" json:"movies"`
	Users  []*User  `gorm:"many2many:user_genres" json:"users"`
}
