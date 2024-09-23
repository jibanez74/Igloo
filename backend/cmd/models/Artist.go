package models

import (
	"errors"

	"gorm.io/gorm"
)

type Artist struct {
	gorm.Model
	Name         string  `gorm:"not null;index" json:"name"`
	OriginalName string  `gorm:"not null;default:'unknown'" json:"originalName"`
	KnownFor     string  `gorm:"not null;default:'unknown'" json:"knownFor"`
	Thumb        string  `gorm:"not null;default:'no_thumb.png'" json:"thumb"`
	Crew         []*Crew `josn:"crew"`
	Cast         []*Cast `json:"cast"`
}

func (a *Artist) BeforeSave(tx *gorm.DB) error {
	if a.Name == "" {
		return tx.AddError(errors.New("artist name is required"))
	}

	return nil
}
