package models

import "gorm.io/gorm"

type Subtitles struct {
	gorm.Model
	Title    string `gorm:"not null;default:'unknown'" json:"title"`
	Index    int64  `json:"index"`
	Codec    string `gorm:"not null;default:'unknown'" json:"codec"`
	Language string `gorm:"not null;default:'unknown'" json:"language"`
	MovieID  uint   `json:"movieID"`
	Movie    Movie  `json:"movie"`
}
