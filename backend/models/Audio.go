package models

import "gorm.io/gorm"

type Audio struct {
	gorm.Model
	Title         string `gorm:"not null;default:'unknown'"`
	IsDefault     bool   `gorm:"default:false"`
	Codec         string `gorm:"not null;default:'unknown'`
	Language      string `gorm:"not null;default:'unknown'`
	Channels      float32
	ChannelLayout string `gorm:"not null;default:'unknown'"`
	Bitrate       uint
	Movie         Movie
	MovieID       uint
}
