package models

import "gorm.io/gorm"

type AudioStream struct {
	gorm.Model
	Title         string `gorm:"not null;default:'unknown'" json:"title"`
	Index         uint   `json:"index"`
	Profile       string `gorm:"not null;default'unknown'" json:"profile"`
	Codec         string `gorm:"not null;default:'unknown'" json:"codec"`
	Channels      uint   `json:"channels"`
	ChannelLayout string `gorm:"not null;default:'unknown'" json:"channelLayout"`
	Language      string `gorm:"not null;default:'unknown'" json:"language"`
	MovieID       uint   `json:"movieID"`
	Movie         Movie  `json:"movie"`
}
