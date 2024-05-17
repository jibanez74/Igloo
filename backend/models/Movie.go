package models

import (
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title           string      `gorm:"not null;index" json:"title"`
	FilePath        string      `gorm:"not null;uniqueIndex" json:"filePath"`
	TagLine         string      `json:"tagLine"`
	Summary         string      `gorm:"type:text" json:"summary"`
	Thumb           string      `gorm:"default:'no_thumb.png'" json:"thumb"`
	Art             string      `gorm:"default:'no_art.png'" json:"art"`
	TmdbID          string      `json:"tmdbID"`
	ImdbID          string      `json:"imdbID"`
	Year            uint        `json:"year"`
	ReleaseDate     time.Time   `json:"releaseDate"`
	Budget          uint        `json:"budget"`
	Revenue         uint        `json:"revenue"`
	ContentRating   string      `json:"contentRating"`
	AudienceRating  float32     `gorm:"default:0" json:"audienceRating"`
	CriticRating    float32     `gorm:"default:0" json:"criticRating"`
	SpokenLanguages string      `json:"spokenLanguages"`
	Resolution      string      `json:"resolution"`
	Genres          []*Genre    `gorm:"many2many:movie_genres" json:"genres"`
	Studios         []*Studio   `gorm:"many2many:movie_studios" json:"studios"`
	Casts           []Cast      `json:"casts"`
	Crew            []Crew      `json:"crew"`
	Trailers        []Trailer   `json:"trailers"`
	VideoList       []Video     `json:"videoList"`
	AudioList       []Audio     `json:"audioList"`
	SubtitleList    []Subtitles `json:"subtitles"`
}
