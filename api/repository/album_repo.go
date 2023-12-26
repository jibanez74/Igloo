package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type albumRepoInterface interface {
	GetAlbums(*[]models.Album) error
	GetAlbumByName(*models.Album, *string) error
	GetAlbumById(album *models.Album, id *int) error
	CreateAlbum(album *models.Album) error
	DeleteAlbum(album *models.Album) error
}

type albumRepo struct {
	db *gorm.DB
}

func NewAlbumRepo(db *gorm.DB) albumRepoInterface {
	return &albumRepo{db: db}
}

func (r *albumRepo) GetAlbums(albums *[]models.Album) error {
	return r.db.Find(albums).Error
}

func (r *albumRepo) GetAlbumByName(album *models.Album, name *string) error {
	return r.db.Where("name = ?", name).First(album).Error
}

func (r *albumRepo) GetAlbumById(album *models.Album, id *int) error {
	return r.db.First(album, id).Error
}

func (r *albumRepo) CreateAlbum(album *models.Album) error {
	return r.db.Create(album).Error
}

func (r *albumRepo) DeleteAlbum(album *models.Album) error {
	return r.db.Delete(album).Error
}
