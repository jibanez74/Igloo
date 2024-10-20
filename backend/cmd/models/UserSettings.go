package models

import (
	"gorm.io/gorm"
)

type UserSettings struct {
	gorm.Model
	TranscodeLimit uint   `gorm:"default:0" json:"transcodeLimit"`
	MovieDir       string `gorm:"not null;default:'/movies'" json:"movieDir"`
	MusicDir       string `gorm:"not null;default:'/music'" json:"musicDir"`
	TVShowsDir     string `gorm:"not null;default:'/tvshows'" json:"tvshowsDir"`
	PhotosDir      string `gorm:"not null;default:'/photos'" json:"photosDir"`
	DownloadImages bool   `gorm:"default:false" json:"downloadImages"`
	UserID         uint   `json:"userID"`
	User           User   `json:"user"`
}
