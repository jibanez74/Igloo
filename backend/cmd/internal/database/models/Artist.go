package models

import (
	"gorm.io/gorm"
)

type Artist struct {
	gorm.Model
	Name         string  `gorm:"not null;default:'unknown';index" json:"name"`
	OriginalName string  `gorm:"not null;default:'unknown'" json:"originalName"`
	KnownFor     string  `gorm:"not null;default:'unknown'" json:"knownFor"`
	Thumb        string  `gorm:"not null;default:'no_thumb.png'" json:"thumb"`
	Crew         []*Crew `json:"crew"`
	Cast         []*Cast `json:"cast"`
}
