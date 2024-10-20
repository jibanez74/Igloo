package models

import (
	"errors"

	"gorm.io/gorm"
)

type Musician struct {
	gorm.Model
	Name    string   `gorm:"not null;index" json:"name"`
	Art     string   `gorm:"not null;default:'no_art.png'" json:"art"`
	Thumb   string   `gorm:"not null;default:'unknown'" json:"thumb"`
	Summary string   `gorm:"type:text; not null; default:'unknown'" json:"summary"`
	Genres  []*Genre `gorm:"many2many:musician_genres" json:"genres"`
	Albums  []*Album `gorm:"many2many:musician_albums" json:"albums"`
	Tracks  []*Track `gorm:"many2many:musician_tracks" json:"tracks"`
	Users   []*User  `gorm:"many2many:user_musicians;" json:"users"`
}

func (m *Musician) BeforeSave(tx *gorm.DB) error {
	if m.Name == "" {
		return tx.AddError(errors.New("musician name is required"))
	}

	return nil
}
