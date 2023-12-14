package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type MusicianRepository interface {
	GetMusicians() ([]models.Musician, error)
	FindByID(id int) (*models.Musician, error)
	FindByName(name string) (*models.Musician, error)
	CreateMusician(musician *models.Musician) error
}

type musicianRepo struct {
	db *gorm.DB
}

func NewMusicianRepository(db *gorm.DB) MusicianRepository {
	return &musicianRepo{
		db: db,
	}
}

func (m *musicianRepo) GetMusicians() ([]models.Musician, error) {
	var musicians []models.Musician

	if err := m.db.Find(&musicians).Error; err != nil {
		return nil, err
	}

	return musicians, nil
}

func (m *musicianRepo) FindByName(name string) (*models.Musician, error) {
	var musician models.Musician

	if err := m.db.Where("name = ?", name).First(&musician).Error; err != nil {
		return nil, err
	}

	return &musician, nil
}

func (m *musicianRepo) FindByID(id int) (*models.Musician, error) {
	var musician models.Musician

	if err := m.db.First(&musician, id).Error; err != nil {
		return nil, err
	}

	return &musician, nil
}

func (m *musicianRepo) CreateMusician(musician *models.Musician) error {
	if err := m.db.Create(&musician).Error; err != nil {
		return err
	}

	return nil
}
