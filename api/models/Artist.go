package models

import "gorm.io/gorm"

type Artist struct {
	gorm.Model
	Name         string `gorm:"size:80;not null;index" json:"name"`
	OriginalName string `gorm:"size:80" json:"originalName"`
	Thumb        string `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumb"`
	KnownFor     string `gorm:"size:20;default:'unknown'" json:"knownFor"`
}
