package models

import "gorm.io/gorm"

type Audio struct {
	gorm.Model
	Title          string  `gorm:"not null" json:"title"`
	IsDefault      bool    `gorm:"default:false" json:"isDefault"`
	Codec          string  `json:"codec"`
	Container      string  `json:"container"`
	Language       string  `json:"language"`
	Channels       float32 `json:"channels"`
	ChannelsLayout string  `json:"channelsLayout"`
	Bitrate        uint    `json:"bitrate"`
	Movie          Movie
	MovieID        uint
}
