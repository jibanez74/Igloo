package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type musicianRepoInterface interface {
	GetMusicians(*[]models.Musician) error
	GetTotalMusicians(count *int64) error
	GetMusicianByName(musicians *models.Musician, name *string) error
	GetMusicianById(musician *models.Musician, id *int) error
	CreateMusician(musician *models.Musician) error
	DeleteMusician(musician *models.Musician) error
}

type musicianRepo struct {
	db *gorm.DB
}

func NewMusicianRepo(db *gorm.DB) musicianRepoInterface {
	return &musicianRepo{db: db}
}

func (r *musicianRepo) GetMusicians(musicians *[]models.Musician) error {
	return r.db.Find(musicians).Error
}

func (r *musicianRepo) GetTotalMusicians(count *int64) error {
	return r.db.Model(&models.Musician{}).Count(count).Error
}

func (r *musicianRepo) GetMusicianByName(musicians *models.Musician, name *string) error {
	return r.db.Where("name = ?", name).First(&musicians).Error
}

func (r *musicianRepo) GetMusicianById(musician *models.Musician, id *int) error {
	return r.db.First(musician, id).Error
}

func (r *musicianRepo) CreateMusician(musician *models.Musician) error {
	return r.db.Create(musician).Error
}

func (r *musicianRepo) DeleteMusician(musician *models.Musician) error {
	return r.db.Delete(&musician).Error
}
