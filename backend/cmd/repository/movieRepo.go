package repository

import (
	"igloo/cmd/database/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (r *repo) MovieExist(title string) (bool, error) {
	var movie struct {
		ID    uint
		Title string
	}

	err := r.db.Model(&models.Movie{}).Where("title = ?", title).Select("id, title").First(&movie).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (r *repo) GetMovieCount() (int64, error) {
	var count int64

	err := r.db.Model(&models.Movie{}).Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *repo) GetMoviesWithPagination(movies *[]SimpleMovie, limit int, offset int) (int, error) {
	err := r.db.Model(&models.Movie{}).Select("id, title, thumb, year").Limit(limit).Offset(offset).Order("created_at desc").Find(&movies).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return fiber.StatusOK, nil
}

func (r *repo) GetMovieByID(movie *models.Movie) (int, error) {
	err := r.db.Preload("CastList.Artist").Preload("CrewList.Artist").Preload("Studios").Preload("VideoList").Preload("AudioList").Preload("SubtitleList").Preload("ChapterList").Preload("Extras").Preload("Genres").First(&movie, movie.ID).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return fiber.StatusOK, nil
}

func (r *repo) CreateMovie(movie *models.Movie) (int, error) {
	err := r.db.Create(&movie).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return fiber.StatusCreated, nil
}
