package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type musicGenreRepoInterface interface {
	GetMusicGenres(*[]models.MusicGenre) error
	GetMusicGenreByName(*models.MusicGenre, *string) error
	GetMusicGenreById(musicGenre *models.MusicGenre, id *int) error
	FindOrCreateMusicGenre(*models.MusicGenre) error
	DeleteMusicGenre(*models.MusicGenre) error
}

type musicGenreRepo struct {
	db *gorm.DB
}

func NewMusicGenreRepo(db *gorm.DB) musicGenreRepoInterface {
	return &musicGenreRepo{db: db}
}

func (r *musicGenreRepo) GetMusicGenres(musicGenres *[]models.MusicGenre) error {
	return r.db.Find(musicGenres).Error
}

func (r *musicGenreRepo) GetMusicGenreByName(musicGenre *models.MusicGenre, name *string) error {
	return r.db.Where("name = ?", name).First(musicGenre).Error
}

func (r *musicGenreRepo) GetMusicGenreById(musicGenre *models.MusicGenre, id *int) error {
	return r.db.Where("id = ?", id).First(musicGenre).Error
}

func (r *musicGenreRepo) FindOrCreateMusicGenre(musicGenre *models.MusicGenre) error {
	return r.db.Where("Tag = ?", musicGenre.Tag).FirstOrCreate(musicGenre).Error
}

func (r *musicGenreRepo) DeleteMusicGenre(musicGenre *models.MusicGenre) error {
	return r.db.Delete(musicGenre).Error
}
