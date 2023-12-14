package repository

import (
  "igloo/models"

  "gorm.io/gorm"
)

type MusicGenreRepository interface {
  FindOrInsertByTag(musicGenre *models.MusicGenre) error
  FindByID(id int) (*models.MusicGenre, error)
}

type musicGenreRepo struct {
  db *gorm.DB
}

func NewMusicGenreRepository(db *gorm.DB) MusicGenreRepository {
  return &musicGenreRepo{
    db: db,
  }
}

func (m *musicGenreRepo) FindOrInsertByTag(musicGenre *models.MusicGenre) error {
  return m.db.Where("tag = ?", musicGenre.Tag).FirstOrCreate(musicGenre).Error
}

func (m *musicGenreRepo) FindByID(id int) (*models.MusicGenre, error) {
  var musicGenre models.MusicGenre

  if err := m.db.First(&musicGenre, id).Error; err != nil {
    return nil, err
  }

  return &musicGenre, nil
}
