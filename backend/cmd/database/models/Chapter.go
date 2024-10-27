package models

import "gorm.io/gorm"

type Chapter struct {
	gorm.Model
	Title     string `gorm:"not null;default:'unknown'" json:"title"`
	Start     uint   `json:"start"`
	StartTime string `json:"startTime"`
	End       uint   `json:"end"`
	EndTime   string `json:"endTime"`
	Thumb     string `gorm:"not null;default:'no_thumb.png'" json:"thumb"`
	MovieID   uint   `json:"movieID"`
	Movie     Movie  `json:"movie"`
}
