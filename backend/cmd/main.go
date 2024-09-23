package main

import (
	"igloo/cmd/models"
	"log"
	"net/http"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	db, err := initDB()
	if err != nil {
		panic(err)
	}

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	app := config{
		DB:       db,
		InfoLog:  infoLog,
		ErrorLog: errorLog,
	}

	err = http.ListenAndServe(os.Getenv("PORT"), app.routes())
	if err != nil {
		errorLog.Fatal(err)
	}
}

func initDB() (*gorm.DB, error) {
	dsn := os.Getenv("DSN")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// run migrations
	db.AutoMigrate(
		&models.User{},
		&models.Artist{},
		&models.Genre{},
		&models.Studio{},
		&models.Movie{},
		&models.Crew{},
		&models.Cast{},
		&models.Trailer{},
		&models.VideoStream{},
		&models.AudioStream{},
		&models.Subtitles{},
		&models.Chapter{},
	)

	return db, nil
}
