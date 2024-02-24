package models

import (
	"gorm.io/gorm"
)

type Musician struct {
	gorm.Model
	Name    string        `gorm:"size:60;not null;index" json:"name"`
	Thumb   string        `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Art     string        `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	Summary string        `gorm:"type:text;default:'unknown'" json:"summary"`
	Genres  []*MusicGenre `gorm:"many2many:musician_genres" json:"genres"`
	Albums  []*Album      `gorm:"many2many:musician_albums" json:"albums"`
	Tracks  []*Track      `gorm:"many2many:musician_tracks" json:"tracks"`
	Users   []*User       `gorm:"many2many:user_musicians;" json:"users"`
}

func (m *Musician) BeforeDelete(tx *gorm.DB) (err error) {
	if err = tx.Model(m).Association("Genres").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Albums").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Tracks").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Users").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Users").Clear(); err != nil {
		return err
	}

	return nil
}
