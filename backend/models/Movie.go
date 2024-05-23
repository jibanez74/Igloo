package models

import (
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title           string `gorm:"not null;index"`
	FilePath        string `gorm:"not null"`
	Size            uint
	Container       string `gorm:"not null;default:'unknown'"`
	Resolution      string `gorm:"not null;default:'unknown'"`
	RunTime         uint
	TagLine         string `gorm:"not null;default:'unknown'"`
	Summary         string `gorm:"type:text;default:'unknown'"`
	Thumb           string `gorm:"not null;default:'no_thumb.png'"`
	Art             string `gorm:"not null;default:'no_art.png'"`
	TmdbID          string `gorn:"not null;default:'unknown';index"`
	ImdbID          string `gorm:"not null;default:'unknown'"`
	Year            uint
	ReleaseDate     time.Time
	Budget          uint
	Revenue         uint
	ContentRating   string `gorm:"not null;default:'unknown'"`
	AudienceRating  float32
	CriticRating    float32
	SpokenLanguages string    `gorm:"not null;default:'unknown'"`
	Genres          []*Genre  `gorm:"many2many:movie_genres"`
	Studios         []*Studio `gorm:"many2many:movie_studios"`
	Casts           []Cast
	Crew            []Crew
	Trailers        []Trailer
	VideoList       []Video
	AudioList       []Audio
	SubtitleList    []Subtitles
	ChapterList     []Chapter
}
