// description: defines the model and hooks for a music mood
package models

import (
	"gorm.io/gorm"
)

type Mood struct {
	gorm.Model
	Tag    string   `gorm:"size:20;not null; unique" json:"tag"`
	Tracks []*Track `gorm:"many2many:track_moods;" json:"tracks"`
}
