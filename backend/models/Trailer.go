package models

import "gorm.io/gorm"

type Trailer struct {
	gorm.Model
	Title   string `gorm:"not nul" json:"title"`
	Url     string `gorm:"not null" json:"url"`
	Movie   Movie  `json:"movie"`
	MovieID uint   `json:"movieID"`
}
