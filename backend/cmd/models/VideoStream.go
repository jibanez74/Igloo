package models

import "gorm.io/gorm"

type VideoStream struct {
	gorm.Model
	Title          string `gorm:"not null;default:'unknown'" json:"title"`
	Index          int64  `json:"index"`
	Profile        string `gorm:"not null;default'unknown'" json:"profile"`
	AspectRatio    string `gorm:"not null;default:'unknown'" json:"aspectRatio"`
	BitDepth       string `gorm:"not null;default:'unknown'" json:"bitDepth"`
	Codec          string `gorm:"not null" json:"codec"`
	Width          int64  `json:"width"`
	Height         int64  `json:"height"`
	CodedWidth     int64  `json:"codedWidth"`
	CodedHeight    int64  `json:"codedHeight"`
	ColorTransfer  string `gorm:"not null;default:'unknown'" json:"colorTransfer"`
	ColorPrimaries string `gorm:"not null;default:unknown'" json:"colorPrimaries"`
	ColorSpace     string `gorm:"not null;default:'unknown'" json:"colorSpace"`
	FrameRate      string `gorm:"not null;default:'unknown'" json:"frameRate"`
	AvgFrameRate   string `gorm:"not nulldefault:'unknown'" json:"avgFrameRate"`
	BPS            int64  `json:"bps"`
	NumberOfBytes  int64  `json:"numberOfBytes"`
	NumberOfFrames int64  `json:"numberOfFrames"`
	MovieID        uint   `json:"movieID"`
	Movie          Movie  `json:"movie"`
}
