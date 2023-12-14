package main

import (
	"igloo/database"
	"igloo/handlers"
	"igloo/repository"
	"net/http"
)

func main() {
	db, err := database.InitDatabase()
	if err != nil {
		panic(err)
	}

	userRepo := repository.NewUserRepository(db)
	userHandler := handlers.NewUserHandler(userRepo)

	moodRepo := repository.NewMoodRepository(db)
	moodHandler := handlers.NewMoodHandler(moodRepo)

	musicGenreRepo := repository.NewMusicGenreRepository(db)
	musicGenreHandler := handlers.NewMusicGenreHandler(musicGenreRepo)

	musicianRepo := repository.NewMusicianRepository(db)
	musicianHandler := handlers.NewMusicianHandler(musicianRepo)

	albumRepo := repository.NewAlbumRepository(db)
	albumHandler := handlers.NewAlbumHandler(albumRepo)

	trackRepo := repository.NewTrackRepository(db)
	trackHandler := handlers.NewTrackHandler(trackRepo)

	appHandlers := handlers.AppHandler{
		UserHandler:       userHandler,
		MoodHandler:       moodHandler,
		MusicGenreHandler: musicGenreHandler,
		MusicianHandler:   musicianHandler,
		AlbumHandler:      albumHandler,
		TrackHandler:      trackHandler,
	}

	err = http.ListenAndServe(":8080", routes(&appHandlers))
	if err != nil {
		panic(err)
	}
}
