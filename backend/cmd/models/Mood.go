package models

import (
	"errors"

	"gorm.io/gorm"
)

type Mood struct {
	gorm.Model
	Tag    string   `gorm:"not null;uniqueIndex" json:"tag"`
	Tracks []*Track `gorm:"many2many:track_moods;" json:"tracks"`
}

func (m *Mood) BeforeSave(tx *gorm.DB) error {
	if m.Tag == "" {
		return tx.AddError(errors.New("mood tag is required"))
	}

	return nil
}
