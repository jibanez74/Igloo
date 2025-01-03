package database

import (
	"igloo/cmd/database/models"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func New() (*gorm.DB, error) {
	dsn := os.Getenv("DSN")

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	gormDB.AutoMigrate(
		&models.GlobalSettings{},
		&models.Movie{},
		&models.Artist{},
		&models.Cast{},
		&models.Crew{},
		&models.Genre{},
		&models.Studio{},
		&models.MovieExtra{},
		&models.Chapter{},
		&models.Subtitles{},
		&models.AudioStream{},
		&models.VideoStream{},
		&models.User{},
	)

	err = createDefaultSettings(gormDB)
	if err != nil {
		return nil, err
	}

	err = createDefaultUser(gormDB)
	if err != nil {
		return nil, err
	}

	return gormDB, nil
}

func createDefaultUser(db *gorm.DB) error {
	var count int64

	err := db.Model(&models.User{}).Count(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		defaultUser := &models.User{
			Name:     "Admin",
			Username: "admin",
			Email:    "admin@example.com",
			Password: "AdminPassword",
			IsAdmin:  true,
			IsActive: true,
		}

		err = db.Create(defaultUser).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func createDefaultSettings(db *gorm.DB) error {
	var count int64

	err := db.Model(&models.GlobalSettings{}).Count(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		workDir, err := os.Getwd()
		if err != nil {
			return err
		}

		defaultSettings := &models.GlobalSettings{
			TranscodeDir:     filepath.Join(workDir, "transcode"),
			MovieDir:         filepath.Join(workDir, "movies"),
			MusicDir:         filepath.Join(workDir, "music"),
			TVShowsDir:       filepath.Join(workDir, "tvshows"),
			PhotosDir:        filepath.Join(workDir, "photos"),
			StaticDir:        filepath.Join(workDir, "static"),
			MaxUserTranscode: 5,
			TmdbKey:          os.Getenv("TMDB_KEY"), // Get from environment variable
		}

		// Create the directories
		dirs := []string{
			defaultSettings.TranscodeDir,
			defaultSettings.MovieDir,
			defaultSettings.MusicDir,
			defaultSettings.TVShowsDir,
			defaultSettings.PhotosDir,
			defaultSettings.StaticDir,
			filepath.Join(defaultSettings.StaticDir, "images"),
			filepath.Join(defaultSettings.StaticDir, "images", "movies"),
			filepath.Join(defaultSettings.StaticDir, "images", "movies", "thumb"),
			filepath.Join(defaultSettings.StaticDir, "images", "movies", "art"),
		}

		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}

		err = db.Create(defaultSettings).Error
		if err != nil {
			return err
		}
	}

	return nil
}
