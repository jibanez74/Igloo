package models

import (
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title       string    `gorm:"size:80;not null;index" json:"title"`
	TmdbID      string    `gorm:"not null;default:'unknown';uniqueIndex" json:"tmdbID"`
	Thumb       string    `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Art         string    `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	Summary     string    `gorm:"type:text" json:"summary"`
	ReleaseDate time.Time `json:"releaseDate"`
	Year        uint      `json:"year"`
	Studio
	Genres []*MovieGenre `gorm:"many2many:movie_genres" json:"genres"`
	Users  []*User       `gorm:"many2many:user_movies" json:"users"`
	Videos []Video       `gorm:"constraint:OnDelete:CASCADE" json:"videos"`
}
