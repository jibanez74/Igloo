package models

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	Title          string `gorm:"not null;default:'unknown'"`
	Index          uint
	Duration       float64
	BitRate        string
	BitDepth       string
	Codec          string `gorm:"not null;default:'unknown'"`
	ColorSpace     string `gorm:"not null;default:'unknown'"`
	ColorPrimaries string `gorm:"not null;default:'unknown'"`
	Width          uint
	Height         uint
	CodedHeight    uint
	CodedWidth     uint
	AvgFrameRate   string `gorm:"not null;default:'unknown'"`
	FrameRate      string `gorm:"not null;default:'unknown'"`
	AspectRatio    string `gorm:"not null;default:'unknown'"`
	NumberOfFrames string
	NumberOfBytes  string
	Chapters       []Chapter
	MovieID        uint
	Movie          Movie
}
