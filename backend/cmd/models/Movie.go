package models

import (
	"os"
	"time"

	"gorm.io/gorm"
)

type Movie struct {
	gorm.Model
	Title           string        `gorm:"not null;index" json:"title"`
	FilePath        string        `gorm:"not null;uniqueIndex" json:"filePath"`
	Size            uint          `json:"size"`
	Container       string        `gorm:"not null;default:'unknown'" json:"container"`
	Resolution      string        `gorm:"not null;default:'unknown'" json:"resolution"`
	RunTime         uint          `json:"runTime"`
	TagLine         string        `gorm:"not null;default:'unknown'" json:"tagLine"`
	Summary         string        `gorm:"type:text;not null;default:'unknown'" json:"summary"`
	Art             string        `gorm:"not null;default:'no_art.png'" json:"art"`
	Thumb           string        `gorm:"not null;default:'unknown'" json:"thumb"`
	TmdbID          string        `gorm:"not null;default:'unknown'" json:"tmdbID"`
	ImdbID          string        `gorm:"not null;default'unknown'" json:"imdbID"`
	Year            int64         `json:"year"`
	ReleaseDate     time.Time     `json:"releaseDate"`
	Budget          uint          `json:"budget"`
	Revenue         uint          `json:"revenue"`
	ContentRating   string        `gorm:"not null;default:'unknown'" json:"contentRating"`
	AudienceRating  float32       `gorm:"default:0" json:"audienceRating"`
	CriticRating    float32       `gorm:"default:0" json:"criticRating"`
	SpokenLanguages string        `gorm:"not null;default:'unknown'" json:"spokenLanguages"`
	Trailers        []Trailer     `json:"trailers"`
	Studios         []*Studio     `gorm:"many2many;movie_studios" json:"studios"`
	Genres          []*Genre      `gorm:"many2many;track_genres" json:"genres"`
	CastList        []Cast        `json:"castList"`
	CrewList        []Crew        `json:"crewList"`
	VideoList       []VideoStream `json:"videoList"`
	AudioList       []AudioStream `json:"audioList"`
	SubtitleList    []Subtitles   `json:"subtitleList"`
	ChapterList     []Chapter     `json:"chapters"`
}

func (m *Movie) BeforeSave(tx *gorm.DB) error {
	info, err := os.Stat(m.FilePath)
	if err != nil {
		return tx.AddError(err)
	}

	if m.Title == "" {
		m.Title = info.Name()
	}

	m.Size = uint(info.Size())

	return nil
}
