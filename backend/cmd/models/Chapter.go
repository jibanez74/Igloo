package models

import "gorm.io/gorm"

type Chapter struct {
	gorm.Model
	Title     string `gorm:"not null;default:'unknown'"`
	TimeBase  string `json:"time_base"`
	Start     uint
	StartTime string `gorm:"not null;default:'unknown'"`
	End       uint
	EndTime   string `json:"end_time"`
	Thumb     string `gorm:"not null;default:'no_chapter_thumb.png'"`
	VideoID   uint
	Video     Video
}
