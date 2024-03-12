package models

import (
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title           string        `gorm:"size:80;not null;index" json:"title"`
	TmdbID          string        `gorm:"not null;default:'unknown';index" json:"tmdbID"`
	ImdbID          string        `gorm:"not null;default:'unknown';index" json:"imdbID"`
	Thumb           string        `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Art             string        `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	Tagline         string        `gorm:"default:'unknown'" json:"tagline"`
	Summary         string        `gorm:"type:text" json:"summary"`
	Budget          uint          `json:"budget"`
	Revenue         uint          `json:"revenue"`
	ReleaseDate     time.Time     `json:"releaseDate"`
	Year            uint          `json:"year"`
	ContentRating   string        `gorm:"size:20" json:"contentRating"`
	AudienceRating  float32       `gorm:"default:0.0" json:"audienceRating"`
	CriticRating    float32       `gorm:"default:0.0" json:"criticRating"`
	Duration        uint          `json:"duration"`
	SpokenLanguages string        `gorm:"default:'unknown'" json:"spokenLanguages"`
	Resolution      string        `gorm:"size:12" json:"resolution"`
	Artists         []*Artist     `gorm:"many2many:artist_movies" json:"artists"`
	Studios         []*Studio     `gorm:"many2many:movie_studios" json:"studios"`
	Genres          []*MovieGenre `gorm:"many2many:movie_genre" json:"genres"`
	Users           []*User       `gorm:"many2many:user_movie" json:"users"`
}
