package repository

import (
	"igloo/cmd/database/models"

	"net/http"
	"strconv"
	"fmt"

	"gorm.io/gorm"
)

func (r *repo) GetLatestMovies(movies *[]SimpleMovie) (int, error) {
	err := r.db.Model(&models.Movie{}).Select("id, title, thumb, art, year").Limit(12).Order("created_at desc").Find(&movies).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return http.StatusOK, nil
}

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

	return http.StatusOK, nil
}

func (r *repo) GetMovieByID(movie *models.Movie) (int, error) {
	err := r.db.Preload("CastList.Artist").Preload("CrewList.Artist").Preload("Studios").Preload("VideoList").Preload("AudioList").Preload("SubtitleList").Preload("ChapterList").Preload("Extras").Preload("Genres").First(&movie, movie.ID).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return http.StatusOK, nil
}

func (r *repo) CreateMovie(movie *models.Movie) (int, error) {
	err := r.db.Create(&movie).Error
	if err != nil {
		return r.gormStatusCode(err), err
	}

	return http.StatusCreated, nil
}

func (r *repo) GetMoviesWithCursor(movies *[]SimpleMovie, cursor string, limit int) (string, error) {
	query := r.db.Model(&models.Movie{}).Select("id, title, thumb, year")
	
	// If cursor is provided, get movies after this ID
	if cursor != "" {
		cursorID, err := strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			return "", err
		}
		query = query.Where("id < ?", cursorID)
	}

	// Get one extra item to determine if there's a next page
	err := query.Order("id desc").Limit(limit + 1).Find(&movies).Error
	if err != nil {
		return "", err
	}

	// If we got more items than limit, there's a next page
	var nextCursor string
	if len(*movies) > limit {
		// Remove the extra item
		lastItem := (*movies)[len(*movies)-1]
		*movies = (*movies)[:limit]
		// Use the ID of the last item as the next cursor
		nextCursor = fmt.Sprintf("%d", lastItem.ID)
	}

	return nextCursor, nil
}
