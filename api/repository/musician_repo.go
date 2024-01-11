package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type MusicianRepo interface {
	GetMusicians(*[]models.Musician) error
	GetMusicianByID(*models.Musician, uint) error
	CreateMusician(*models.Musician) error
}

type musicianRepo struct {
	db *gorm.DB
}

func NewMusicianRepo(db *gorm.DB) *musicianRepo {
	return &musicianRepo{db}
}

func (r *musicianRepo) GetMusicians(m *[]models.Musician) error {
	return r.db.Find(m).Error
}

func (r *musicianRepo) GetMusicianByID(m *models.Musician, id uint) error {
	return r.db.First(m, id).Error
}

func (r *musicianRepo) CreateMusician(m *models.Musician) error {
	return r.db.Create(m).Error
}
