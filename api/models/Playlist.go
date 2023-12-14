// description: defines the playlist model and its hooks
package models

import (
	"gorm.io/gorm"
)

type Playlist struct {
	gorm.Model
	Title     string `gorm:"size:20; not null" json:"title"`
	Summary   string `gorm:"size:255; not null; default:'unknown'" json:"summary"`
	Thumbnail string `gorm:"size:255; not null; default:'playlist.jpg'" json:"thumbnail"`
	Art       string `gorm:"size:255; not null; default:'playlist.jpg'" json:"art"`
	UserID    uint
	User      User
	Tracks    []*Track `gorm:"many2many:playlist_tracks" json:"tracks"`
}
