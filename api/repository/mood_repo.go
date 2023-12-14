// description: defines the repo for a music mood
package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type MoodRepository interface {
	FindOrInsertByTag(mood *models.Mood) error
	FindByID(id int) (*models.Mood, error)
}

type moodRepo struct {
	db *gorm.DB
}

func NewMoodRepository(db *gorm.DB) MoodRepository {
	return &moodRepo{
		db: db,
	}
}

func (m *moodRepo) FindOrInsertByTag(mood *models.Mood) error {
	return m.db.Where("tag = ?", mood.Tag).FirstOrCreate(mood).Error
}

func (m *moodRepo) FindByID(id int) (*models.Mood, error) {
	var mood models.Mood

	if err := m.db.First(&mood, id).Error; err != nil {
		return nil, err
	}

	return &mood, nil
}
