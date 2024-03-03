package database

import (
	"igloo/models"

	"gorm.io/gorm"
)

func RunMigrations(db *gorm.DB) {
	err := db.AutoMigrate(
		&models.Album{},
		&models.Musician{},
		&models.MusicGenre{},
		&models.MusicMood{},
		&models.Track{},
		&models.TrackHistory{},
		&models.Playlist{},

		// movie data migrations
		&models.Artist{},
		&models.CastMember{},
		&models.CrewMember{},
		&models.Studio{},
		&models.MovieGenre{},
		&models.Movie{},

		// user data migrations
		&models.User{},
	)

	if err != nil {
		panic("Error: Unable to run user migrations")
	}

	if err != nil {
		panic("Error: Unable to run user migrations")
	}

}
