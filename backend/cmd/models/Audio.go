package models

import "gorm.io/gorm"

type Audio struct {
	gorm.Model
	Title         string `gorm:"not null;default:'unknown'"`
	Index         uint
	Channels      uint
	ChannelLayout string `gorm:"not null;default:'unknown'"`
	Codec         string `gorm:"not null;default:'unknown'"`
	Profile       string `gorm:"not null;default:'unknown'"`
	BitRate       string `gorm:"not null;default:'unknown'"`
	Language      string `gorm:"not null;default:'unknwon'"`
	Movie         Movie
	MovieID       uint
}
