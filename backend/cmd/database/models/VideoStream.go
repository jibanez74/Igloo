package models

import "gorm.io/gorm"

type VideoStream struct {
	gorm.Model
	Title          string `gorm:"not null;default:'unknown'" json:"title"`
	Index          uint   `json:"index"`
	Profile        string `gorm:"not null;default'unknown'" json:"profile"`
	AspectRatio    string `gorm:"not null;default:'unknown'" json:"aspectRatio"`
	BitRate        string `gorm:"not null;default:'unknown'" json:"bitRate"`
	BitDepth       string `gorm:"not null;default:'unknown'" json:"bitDepth"`
	Codec          string `gorm:"not null" json:"codec"`
	Width          uint   `json:"width"`
	Height         uint   `json:"height"`
	CodedWidth     uint   `json:"codedWidth"`
	CodedHeight    uint   `json:"codedHeight"`
	ColorTransfer  string `gorm:"not null;default:'unknown'" json:"colorTransfer"`
	ColorPrimaries string `gorm:"not null;default:unknown'" json:"colorPrimaries"`
	ColorSpace     string `gorm:"not null;default:'unknown'" json:"colorSpace"`
	ColorRange     string `gorm:"not null;default:'unknown'" json:"colorRange"`
	FrameRate      string `gorm:"not null;default:'unknown'" json:"frameRate"`
	AvgFrameRate   string `gorm:"not nulldefault:'unknown'" json:"avgFrameRate"`
	Level          int    `json:"level"`
	MovieID        uint   `json:"movieID"`
	Movie          Movie  `json:"movie"`
}
