package models

import "gorm.io/gorm"

type Subtitles struct {
	gorm.Model
	Title    string `gorm:"not null" json:"title"`
	Language string `gorm:"not null" json:"language"`
	Codec    string `gorm:"not null" json:"codec"`
	MovieID  uint
	Movie    Movie
}
