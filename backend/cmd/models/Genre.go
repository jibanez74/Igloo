package models

import (
	"errors"

	"gorm.io/gorm"
)

type Genre struct {
	gorm.Model
	Tag       string   `gorm:"not null;index" json:"tag"`
	GenreType string   `gorm:"not null" json:"genreType"`
	Movies    []*Movie `gorm:"many2many;movie_genres" json:"movies"`
}

func (g *Genre) BeforeSave(tx *gorm.DB) error {
	if g.Tag == "" {
		return tx.AddError(errors.New("genre tag is required"))
	}

	if g.GenreType != "movie" && g.GenreType != "music" {
		return tx.AddError(errors.New("genre type is invalid"))
	}

	return nil
}
