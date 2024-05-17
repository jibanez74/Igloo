package models

import "gorm.io/gorm"

type Artist struct {
	gorm.Model
	Name         string `gorm:"not null;index" json:"name"`
	OriginalName string `json:"originalName"`
	Thumb        string `gorm:"default:'no_thumb.png'" json:"thumb"`
	KnownFor     string `json:"knownFor"`
}
