package repository

import (
	"igloo/models"

	"gorm.io/gorm"
)

type AlbumRepo interface {
	GetAlbums(*[]models.Album) error
	GetAlbumByID(*models.Album, uint) error
	GetAlbumByTitle(*models.Album, string) error
	CreateAlbum(*models.Album) error
}

type albumRepo struct {
	db *gorm.DB
}

func NewAlbumRepo(db *gorm.DB) *albumRepo {
	return &albumRepo{db}
}

func (r *albumRepo) GetAlbums(a *[]models.Album) error {
	return r.db.Find(a).Error
}

func (r *albumRepo) GetAlbumByID(a *models.Album, id uint) error {
	return r.db.First(a, id).Error
}

func (r *albumRepo) GetAlbumByTitle(a *models.Album, t string) error {
	return r.db.Where("Title = ?", t).Error
}

func (r *albumRepo) CreateAlbum(a *models.Album) error {
	return r.db.Create(a).Error
}
