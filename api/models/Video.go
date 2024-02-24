package models

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	Title   string `gorm:"size:80;not null;index" json:"title"`
	MovieID *uint
	Movie   Movie `json:"movie"`
}
