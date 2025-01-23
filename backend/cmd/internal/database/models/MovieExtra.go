package models

import (
	"errors"

	"gorm.io/gorm"
)

type MovieExtra struct {
	gorm.Model
	Title   string `gorm:"not null;default'unknown'" json:"title"`
	Url     string `gorm:"not null" json:"url"`
	Kind    string `gorm:"not null;default:'unknown'" json:"kind"`
	MovieID uint   `json:"movieID"`
	Movie   Movie  `json:"movie"`
}

func (m *MovieExtra) BeforeSave(tx *gorm.DB) (err error) {
	if m.Url == "" {
		return errors.New("url is required")
	}

	return
}
