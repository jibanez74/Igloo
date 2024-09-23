package models

import (
	"errors"

	"gorm.io/gorm"
)

type Trailer struct {
	gorm.Model
	Title   string `gorm:"not null;default:'unknown'" json:"title"`
	Url     string `gorm:"not null" json:"url"`
	Thumb   string `gorm:"not null;default:'no_thumb.png'" json:"thumb"`
	MovieID uint   `json:"movieID"`
	Movie   Movie  `json:"movie"`
}

func (t *Trailer) BeforeSave(tx *gorm.DB) error {
	if t.Url == "" {
		return tx.AddError(errors.New("trailer url is required"))
	}

	return nil
}
