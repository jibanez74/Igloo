// description: defines the model for play history and its hooks
package models

import (
	"gorm.io/gorm"
)

type TrackHistory struct {
	gorm.Model
	UserID   uint
	User     User
	Tracks   []*Track `gorm:"many2many:track_history" json:"tracks"`
	Liked    bool     `gorm:"default:false; not null" json:"liked"`
	Disliked bool     `gorm:"default:false; not null" json:"disliked"`
}
