package repository

import (
	"igloo/cmd/helpers"
	"igloo/cmd/models"
	"os"
)

func (r *Repo) MovieExistsByTitle(title string) (int64, error) {
	var count int64

	err := r.db.Model(&models.Movie{}).Where("title = ?", title).Count(&count).Error
	if err != nil {
		return count, err
	}

	return count, nil
}

func (r *Repo) GetMovieCount(count *int64) error {
	return r.db.Model(&models.Movie{}).Count(count).Error
}

func (r *Repo) GetLatestMovies(movies *[12]models.RecentMovie) error {
	return r.db.Model(&models.Movie{}).Select("id, title, thumb, year").Limit(12).Order("created_at desc").Find(movies).Error
}

func (r *Repo) GetMovieByID(movie *models.Movie, id uint) error {
	return r.db.Preload("Genres").Preload("Studios").Preload("CastList").Preload("CrewList").Preload("Trailers").Preload("VideoList").Preload("AudioList").Preload("SubtitleList").First(movie, id).Error
}

func (r *Repo) CreateMovie(movie *models.Movie) error {
	info, err := os.Stat(movie.FilePath)
	if err != nil {
		return err
	}

	if movie.Title == "" {
		movie.Title = info.Name()
	}

	movie.Size = uint(info.Size())

	audioList, err := helpers.GetAudioTracks(movie.FilePath)
	if err != nil {
		return err
	}
	movie.AudioList = audioList

	videos, err := helpers.GetVideoTrack(movie.FilePath)
	if err != nil {
		return err
	}
	movie.VideoList = videos

	subtitles, err := helpers.GetSubtitles(movie.FilePath)
	if err != nil {
		return err
	}
	movie.SubtitleList = subtitles

	container, err := helpers.GetContainerFormat(movie.FilePath)
	if err != nil {
		return err
	}
	movie.Container = container

	return r.db.Create(movie).Error
}
