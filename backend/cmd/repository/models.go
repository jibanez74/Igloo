package repository

import (
	"igloo/cmd/database/models"

	"gorm.io/gorm"
)

type Repo interface {
	gormStatusCode(err error) int
	GetLatestMovies(movies *[]SimpleMovie) (int, error)
	MovieExist(title string) (bool, error)
	GetMovieCount() (int64, error)
	GetMoviesWithPagination(movies *[]SimpleMovie, limit int, offset int) (int, error)
	GetMovieByID(movie *models.Movie) (int, error)
	CreateMovie(*models.Movie) (int, error)

	GetAuthUser(*models.User) (int, error)
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
