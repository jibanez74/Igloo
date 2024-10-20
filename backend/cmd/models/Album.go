package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Album struct {
	gorm.Model
	Title          string      `gorm:"not null;index" json:"title"`
	NumberOfTracks uint        `gorm:"not null; default:0" json:"numberOfTracks"`
	Summary        string      `gorm:"type:text; not null; default:'unknown'" json:"summary"`
	Art            string      `gorm:"not null;default:'no_art.png'" json:"art"`
	Thumb          string      `gorm:"not null;default:'unknown'" json:"thumb"`
	Studio         string      `gorm:" not null" json:"studio"`
	Year           uint        `gorm:"not null; default:0" json:"year"`
	ReleaseDate    time.Time   `json:"releaseDate"`
	Musicians      []*Musician `gorm:"many2many:musician_albums" json:"musicians"`
	Tracks         []Track     `gorm:"constraint:OnDelete:CASCADE" json:"tracks"`
	Genres         []*Genre    `gorm:"many2many:album_genres;" json:"genres"`
}

func (a *Album) BeforeSave(tx *gorm.DB) error {
	if a.Title == "" {
		return tx.AddError(errors.New("album title is required"))
	}

	return nil
}
