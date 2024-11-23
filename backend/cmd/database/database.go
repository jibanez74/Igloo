package database

import (
	"igloo/cmd/database/models"
	"os"
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
		&models.UserSettings{},
	)

	return gormDB, nil
}
