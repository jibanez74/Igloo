package models

import (
	"time"

	"gorm.io/gorm"
)

type Album struct {
	gorm.Model
	Title          string        `gorm:"size:60;not null;index" json:"title"`
	NumberOfTracks uint          `gorm:"default:0" json:"numberOfTracks"`
	Summary        string        `gorm:"type:text;default:'unknown'" json:"summary"`
	Thumb          string        `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Art            string        `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	Studio         string        `gorm:"size:60;default:'unknown'" json:"studio"`
	Year           uint          `json:"year"`
	ReleaseDate    time.Time     `json:"releaseDate"`
	Musicians      []*Musician   `gorm:"many2many:musician_albums" json:"musicians"`
	Tracks         []Track       `gorm:"constraint:OnDelete:CASCADE" json:"tracks"`
	Genres         []*MusicGenre `gorm:"many2many:album_genres;" json:"genres"`
}

func (m *Album) BeforeDelete(tx *gorm.DB) (err error) {
	if err = tx.Model(m).Association("Musicians").Clear(); err != nil {
		return err
	}

	if err = tx.Model(m).Association("Genres").Clear(); err != nil {
		return err
	}

	return nil
}
