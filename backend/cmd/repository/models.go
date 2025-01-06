package repository

import (
	"igloo/cmd/database/models"

	"gorm.io/gorm"
)

type Repo interface {
	gormStatusCode(err error) int
	GetSettings() (*models.GlobalSettings, error)
	GetLatestMovies(movies *[]SimpleMovie) (int, error)
	MovieExist(title string) (bool, error)
	GetMovieCount() (int64, error)
	GetMovieByID(movie *models.Movie) (int, error)
	CreateMovie(*models.Movie) (int, error)
	GetAllMovies(movies *[]SimpleMovie) (int, error)

	GetAuthUser(*models.User) error
	GetUserByID(*models.User) (int, error)
	CreateUser(*models.User) (int, error)
}

type repo struct {
	db *gorm.DB
}

type SimpleMovie struct {
	ID    uint
	Title string `json:"title"`
	Thumb string `json:"thumb"`
	Year  uint   `json:"year"`
}
