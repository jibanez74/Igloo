package database

import (
	"igloo/cmd/database/models"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func New() (*gorm.DB, error) {
	dsn := os.Getenv("DSN")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(
		&models.Movie{},
		&models.Artist{},
		&models.Cast{},
		&models.Crew{},
		&models.Genre{},
		&models.Studio{},
		&models.Trailer{},
		&models.Chapter{},
		&models.Subtitles{},
		&models.AudioStream{},
		&models.VideoStream{},
		&models.User{},
		&models.UserSettings{},
	)

	return db, nil
}
