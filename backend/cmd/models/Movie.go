package models

import (
	"os"
	"time"

	"gorm.io/gorm"
)

type SimpleMovie struct {
	ID    uint
	Title string `json:"title"`
	Thumb string `json:"thumb"`
	Year  uint   `json:"year"`
}

type Movie struct {
	gorm.Model
	Title           string        `gorm:"not null;index" json:"title"`
	FilePath        string        `gorm:"not null;uniqueIndex" json:"filePath"`
	Container       string        `gorm:"not null" json:"container"`
	Size            uint          `json:"size"`
	ContentType     string        `gorm:"not null" json:"contentType"`
	Resolution      uint          `gorm:"default:0" json:"resolution"`
	RunTime         uint          `gorm:"default:0" json:"runTime"`
	TagLine         string        `gorm:"not null;default:'unknown'" json:"tagLine"`
	Summary         string        `gorm:"type:text;not null;default:'unknown'" json:"summary"`
	Art             string        `gorm:"not null;default:'no_art.png'" json:"art"`
	Thumb           string        `gorm:"not null;default:'unknown'" json:"thumb"`
	TmdbID          string        `gorm:"not null;default:'unknown'" json:"tmdbID"`
	ImdbID          string        `gorm:"not null;default'unknown'" json:"imdbID"`
	Year            int64         `gorm:"default:0" json:"year"`
	ReleaseDate     time.Time     `json:"releaseDate"`
	Budget          uint          `gorm:"default:0" json:"budget"`
	Revenue         uint          `gorm:"default:0" json:"revenue"`
	ContentRating   string        `gorm:"not null;default:'unknown'" json:"contentRating"`
	AudienceRating  float32       `gorm:"default:0" json:"audienceRating"`
	CriticRating    float32       `gorm:"default:0" json:"criticRating"`
	SpokenLanguages string        `gorm:"not null;default:'unknown'" json:"spokenLanguages"`
	Trailers        []Trailer     `json:"trailers"`
	Studios         []*Studio     `gorm:"many2many:movie_studios" json:"studios"`
	Genres          []*Genre      `gorm:"many2many:movie_genres" json:"genres"`
	CastList        []Cast        `json:"castList"`
	CrewList        []Crew        `json:"crewList"`
	VideoList       []VideoStream `json:"videoList"`
	AudioList       []AudioStream `json:"audioList"`
	SubtitleList    []Subtitles   `json:"subtitleList"`
	ChapterList     []Chapter     `json:"chapters"`
	Users []*User `gorm:"many2many:user_movies" json:"users"`
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
