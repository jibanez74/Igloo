// description: manages the repo for working with tracks
package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type TrackRepository interface {
	Create(t *models.Track) error
	FindByID(id int) (*models.Track, error)
}

type trackRepo struct {
	db *gorm.DB
}

func NewTrackRepository(db *gorm.DB) TrackRepository {
	return &trackRepo{
		db: db,
	}
}

func (r *trackRepo) Create(t *models.Track) error {
	return r.db.Create(t).Error
}

func (r *trackRepo) FindByID(id int) (*models.Track, error) {
	var track models.Track

	if err := r.db.First(&track, id).Error; err != nil {
		return nil, err
	}

	return &track, nil
}
