package models

import (
	"time"

	"gorm.io/gorm"
)

type Album struct {
	gorm.Model
	Title          string    `gorm:"size:60;not null;index" json:"title"`
	NumberOfTracks uint      `gorm:"default:0" json:"numberOfTracks"`
	Summary        string    `gorm:"type:text;default:'unknown'" json:"summary"`
	Thumb          string    `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Art            string    `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	Year           uint      `json:"year"`
	ReleaseDate    time.Time `json:"releaseDate"`
	Musician       Musician
	MusicianID     uint
	Tracks         []Track       `json:"tracks"`
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
