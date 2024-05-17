package models

import "gorm.io/gorm"

type Video struct {
	gorm.Model
	Title       string  `gorm:"not null" json:"title"`
	IsDefault   bool    `gorm:"default:false" json:"isDefault"`
	Duration    uint    `json:"duration"`
	Bitrate     uint    `json:"bitrate"`
	BitDepth    uint    `json:"bitDepth"`
	Codec       string  `json:"codec"`
	Container   string  `json:"container"`
	ColorSpace  string  `json:"colorSpace"`
	Width       uint    `json:"width"`
	Height      uint    `json:"height"`
	FrameRate   float32 `json:"frameRate"`
	AspectRatio uint    `json:"aspectRatio"`
	MovieID     uint
	Movie       Movie
}
