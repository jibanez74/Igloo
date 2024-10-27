package models

import (
	"errors"

	"gorm.io/gorm"
)

type Studio struct {
	gorm.Model
	Name    string   `gorm:"not null;index" json:"name"`
	Country string   `gorm:"not null;default:'unknown'" json:"country"`
	Logo    string   `gorm:"not null;default:'unknown'" json:"logo"`
	Movies  []*Movie `gorm:"many2many;movie_studios" json:"movies"`
}

func (s *Studio) BeforeSave(tx *gorm.DB) error {
	if s.Name == "" {
		return tx.AddError(errors.New("studio name is required"))
	}

	return nil
}
