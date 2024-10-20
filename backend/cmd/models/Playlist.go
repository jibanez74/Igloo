package models

import (
	"errors"

	"gorm.io/gorm"
)

type Playlist struct {
	gorm.Model
	Title   string `gorm:" not null;index" json:"title"`
	Summary string `gorm:"not null;default:'unknown'" json:"summary"`
	Art     string `gorm:"not null;default:'no_art.png'" json:"art"`
	Thumb   string `gorm:"not null;default:'unknown'" json:"thumb"`
	UserID  uint
	User    User
	Tracks  []*Track `gorm:"many2many:playlist_tracks" json:"tracks"`
}

func (p *Playlist) BeforeSave(tx *gorm.DB) error {
	if p.Title == "" {
		return tx.AddError(errors.New("playlist title is required"))
	}

	return nil
}
