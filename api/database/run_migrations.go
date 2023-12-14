// description: This package is responsible for running all the migrations
package database

import (
	"igloo/models"

	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) {
	err := db.AutoMigrate(&models.User{}, &models.MusicGenre{}, &models.Mood{}, &models.Musician{}, &models.Album{}, &models.Track{}, &models.Playlist{}, &models.TrackHistory{})
	if err != nil {
		panic("Error: Unable to run user migrations")
	}

	err = db.AutoMigrate(&models.User{})
	if err != nil {
		panic("Error: Unable to run user migrations")
	}
}
