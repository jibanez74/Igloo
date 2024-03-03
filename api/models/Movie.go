package models

import (
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title          string        `gorm:"size:80;not null;index" json:"title"`
	TmdbID         string        `gorm:"not null;default:'unknown';index" json:"tmdbID"`
	ImdbID         string        `gorm:"not null;default:'unknown';index" json:"imdbID"`
	Thumb          string        `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	Art            string        `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	Tagline        string        `gorm:"size:60" json:"tagline"`
	Summary        string        `gorm:"type:text" json:"summary"`
	Budget         uint64        `json:"budget"`
	Revenue        uint64        `json:"revenue"`
	ReleaseDate    time.Time     `json:"releaseDate"`
	Year           uint          `json:"year"`
	AudienceRating float32       `gorm:"default:0.0" json:"audienceRating"`
	CriticRating   float32       `gorm:"default:0.0" json:"criticRating"`
	Duration       uint          `json:"duration"`
	Videos         []Video       `gorm:"constraint:OnDelete:CASCADE" json:"videos"`
	Cast           []CastMember  `gorm:"constraint:OnDelete:CASCADE" json:"cast"`
	Crew           []CrewMember  `gorm:"constraint:OnDelete:CASCADE" json:"crew"`
	Genres         []*MovieGenre `gorm:"many2many:movie_genre" json:"genres"`
  Users []*User `gorm:"many2many:user_movie" json:"users"`
}
