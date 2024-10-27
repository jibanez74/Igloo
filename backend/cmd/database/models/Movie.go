package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title           string        `gorm:"not null;index" json:"title"`
	FilePath        string        `gorm:"not null;uniqueIndex" json:"filePath"`
	Container       string        `gorm:"not null" json:"container"`
	Size            uint          `json:"size"`
	ContentType     string        `gorm:"not null" json:"contentType"`
	Resolution      string        `gorm:"not null;default:'unknown'" json:"resolution"`
	RunTime         uint          `gorm:"default:0" json:"runTime"`
	Adult           bool          `gorm:"default:false" json:"adult"`
	TagLine         string        `gorm:"not null;default:'unknown'" json:"tagLine"`
	Summary         string        `gorm:"type:text;not null;default:'unknown'" json:"summary"`
	Art             string        `gorm:"not null;default:'no_art.png'" json:"art"`
	Thumb           string        `gorm:"not null;default:'unknown'" json:"thumb"`
	TmdbID          string        `gorm:"not null;default:'unknown'" json:"tmdbID"`
	ImdbUrl         string        `gorm:"not null;default:'unknown'" json:"imdbUrl"`
	Year            uint          `gorm:"default:0" json:"year"`
	ReleaseDate     time.Time     `json:"releaseDate"`
	Budget          uint          `gorm:"default:0" json:"budget"`
	Revenue         uint          `gorm:"default:0" json:"revenue"`
	ContentRating   string        `gorm:"not null;default:'unknown'" json:"contentRating"`
	AudienceRating  float32       `gorm:"default:0" json:"audienceRating"`
	CriticRating    float64       `gorm:"default:0" json:"criticRating"`
	SpokenLanguages string        `gorm:"not null;default:'unknown'" json:"spokenLanguages"`
	Extras          []MovieExtra  `json:"extras"`
	Studios         []*Studio     `gorm:"many2many:movie_studios" json:"studios"`
	Genres          []*Genre      `gorm:"many2many:movie_genres" json:"genres"`
	CastList        []Cast        `json:"castList"`
	CrewList        []Crew        `json:"crewList"`
	VideoList       []VideoStream `json:"videoList"`
	AudioList       []AudioStream `json:"audioList"`
	SubtitleList    []Subtitles   `json:"subtitleList"`
	ChapterList     []Chapter     `json:"chapters"`
}

func (m *Movie) BeforeCreate(tx *gorm.DB) (err error) {
	if m.Title == "" {
		return errors.New("title is required")
	}

	if m.FilePath == "" {
		return errors.New("filePath is required")
	}

	if m.Container == "" {
		return errors.New("container is required")
	}

	if m.ContentType == "" {
		return errors.New("contentType is required")
	}

	return nil
}
