package models

import "gorm.io/gorm"

type GlobalSettings struct {
	gorm.Model
	TranscodeDir     string `gorm:"not null" json:"transcodeDir"`
	MovieDir         string `gorm:"not null" json:"movieDir"`
	MusicDir         string `gorm:"not null" json:"musicDir"`
	TVShowsDir       string `gorm:"not null" json:"tvshowsDir"`
	PhotosDir        string `gorm:"not null" json:"photosDir"`
	StaticDir        string `gorm:"not null" json:"staticDir"`
	MaxUserTranscode uint   `gorm:"default:5" json:"maxUserTranscode"`
	TmdbKey          string `json:"tmdbKey"`
}
