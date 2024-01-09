package models

import (
	"gorm.io/gorm"
)

type MusicGenre struct {
	gorm.Model
	Tag       string      `gorm:"size:20;not null;uniqueIndex" json:"tag"`
	Musicians []*Musician `gorm:"many2many:musician_genres" json:"musicians"`
	Albums    []*Album    `gorm:"many2many:album_genres;" json:"albums"`
	Tracks    []*Track    `gorm:"many2many:track_genres;" json:"tracks"`
	Users     []*User     `gorm:"many2many:user_genres;" json:"users"`
}
