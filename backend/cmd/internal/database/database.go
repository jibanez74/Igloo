package database

import (
	"errors"
	"fmt"
	"igloo/cmd/internal/database/models"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var dbTypes = [2]string{
	"sqlite",
	"postgres",
}

func New() (*gorm.DB, error) {
	var dialector gorm.Dialector
	var gormConfig gorm.Config

	dbType := os.Getenv("DB_TYPE")
	if dbType == "" {
		return nil, errors.New("DB_TYPE environment variable is not set")
	}

	validType := false
	for _, t := range dbTypes {
		if t == dbType {
			validType = true
			break
		}
	}
	if !validType {
		return nil, fmt.Errorf("unsupported database type: %s. Must be one of: %v", dbType, dbTypes)
	}

	switch dbType {
	case "postgres":
		dsn := os.Getenv("DSN")
		if dsn == "" {
			return nil, errors.New("DSN environment variable is not set")
		}

		gormConfig = gorm.Config{
			PrepareStmt: true,
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
			CreateBatchSize: 1000,
		}

		dialector = postgres.Open(dsn)
	case "sqlite":
		gormConfig = gorm.Config{
			PrepareStmt: true,
			NowFunc: func() time.Time {
				return time.Now().UTC()
			},
			CreateBatchSize:                          100,
			DisableForeignKeyConstraintWhenMigrating: true,
		}

		dbPath := filepath.Join("data", "igloo.db")
		if err := os.MkdirAll("data", 0755); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}

		dialector = sqlite.Open(dbPath + "?_journal=WAL&_synchronous=NORMAL&_timeout=5000")
	}

	gormDB, err := gorm.Open(dialector, &gormConfig)
	if err != nil {
		return nil, err
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}

	switch dbType {
	case "postgres":
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	case "sqlite":
		sqlDB.SetMaxIdleConns(1)
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetConnMaxLifetime(0)
	}

	if dbType == "sqlite" {
		gormDB.Exec("PRAGMA journal_mode=WAL;")
		gormDB.Exec("PRAGMA busy_timeout=5000;")
		gormDB.Exec("PRAGMA synchronous=NORMAL;")
		gormDB.Exec("PRAGMA cache_size=-64000;")
		gormDB.Exec("PRAGMA foreign_keys=ON;")
		gormDB.Exec("PRAGMA temp_store=MEMORY;")
	}

	if dbType == "postgres" {
		err = gormDB.AutoMigrate(
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
	} else {
		for _, model := range []interface{}{
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
		} {

			err = gormDB.AutoMigrate(model)
			if err != nil {
				return nil, fmt.Errorf("failed to migrate %T: %w", model, err)
			}
		}
	}

	if err != nil {
		return nil, err
	}

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
			Ffmpeg:           "ffmpeg",
			Ffprobe:          "ffprobe",
			TmdbKey:          os.Getenv("TMDB_API_KEY"),
		}

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
