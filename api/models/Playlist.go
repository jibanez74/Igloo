// description: defines the playlist model and its hooks
package models

import (
	"gorm.io/gorm"
)

type Playlist struct {
	gorm.Model
	Title     string `gorm:"size:60; not null" json:"title"`
	Summary   string `gorm:"size:200;default:'unknown'" json:"summary"`
	Thumbnail string `gorm:"default:'/public/images/no_thumb.jpg'" json:"thumbnail"`
	Art       string `gorm:"default:'/public/images/no_art.jpg'" json:"art"`
	UserID    uint
	User      User
	Tracks    []*Track `gorm:"many2many:playlist_tracks" json:"tracks"`
}
