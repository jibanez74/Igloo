// description: package repository implements the repository pattern for the album model
package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type AlbumRepository interface {
	FindOrInsertByTitle(album *models.Album) error
	FindByID(id int) (*models.Album, error)
}

type albumRepo struct {
	db *gorm.DB
}

func NewAlbumRepository(db *gorm.DB) AlbumRepository {
	return &albumRepo{
		db: db,
	}
}

func (a *albumRepo) FindOrInsertByTitle(album *models.Album) error {
	return a.db.Where("title = ?", album.Title).FirstOrCreate(album).Error
}

func (a *albumRepo) FindByID(id int) (*models.Album, error) {
	var album models.Album

	if err := a.db.First(&album, id).Error; err != nil {
		return nil, err
	}

	return &album, nil
}
