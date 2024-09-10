package models

import "gorm.io/gorm"

type Artist struct {
	gorm.Model
	Name         string `gorm:"not null;index"`
	OriginalName string `gorm:"not null;index"`
	Thumb        string `gorm:"not null;default:'no_thumb.png'"`
	KnownFor     string `gorn:"not null;default:'unknown'"`
}
