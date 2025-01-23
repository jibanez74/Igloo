package models

import "gorm.io/gorm"

type GlobalSettings struct {
	gorm.Model
	Debug            bool   `gorm:"default:false" json:"debug"`
	TranscodeDir     string `gorm:"not null" json:"transcodeDir"`
	MovieDir         string `gorm:"not null" json:"movieDir"`
	MusicDir         string `gorm:"not null" json:"musicDir"`
	TVShowsDir       string `gorm:"not null" json:"tvshowsDir"`
	PhotosDir        string `gorm:"not null" json:"photosDir"`
	StaticDir        string `gorm:"not null" json:"staticDir"`
	MaxUserTranscode uint   `gorm:"default:5" json:"maxUserTranscode"`
	Ffmpeg           string `gorm:"not null" json:"ffmpeg"`
	Ffprobe          string `gorm:"not null" json:"ffprobe"`
	TmdbKey          string `json:"tmdbKey"`
	RedisHost        string `gorm:"not null;default:'localhost'" json:"redisHost"`
	RedisUser        string `gorm:"not null;default:''" json:"redisUser"`
	RedisPassword    string `gorm:"not null;default:''" json:"redisPassword"`
	RedisPort        int    `gorm:"default:6379" json:"redisPort"`
	DownloadImages   bool   `gorm:"default:false" json:"downloadImages"`
}
