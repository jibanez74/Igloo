package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type MusicGenreRepo interface {
	GetMusicGenres(*[]models.MusicGenre) error
	CreateMusicGenre(*models.MusicGenre) error
}

type musicGenreRepo struct {
	db *gorm.DB
}

func NewMusicGenreRepo(db *gorm.DB) *musicGenreRepo {
	return &musicGenreRepo{db}
}

func (r *musicGenreRepo) GetMusicGenres(g *[]models.MusicGenre) error {
	return r.db.Find(g).Error
}

func (r *musicGenreRepo) CreateMusicGenre(g *models.MusicGenre) error {
	return r.db.Create(g).Error
}
