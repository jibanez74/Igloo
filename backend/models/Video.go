package models

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	Title          string `gorm:"not null;default:'unknown'"`
	IsDefault      bool   `gorm:"default:false"`
	Duration       uint
	BitRate        uint
	BitDepth       uint
	Codec          string `gorm:"not null;default:'unknown'"`
	ColorSpace     string `gorm:"not null;default:'unknown'"`
	ColorPrimaries string `gorm:"not null;default:'unknown'"`
	Width          uint
	Height         uint
	AvgFrameRate   float32
	FrameRate      float32
	AspectRatio    string `gorm:"not null;default:'unknown'"`
	MovieID        uint
	Movie          Movie
}
