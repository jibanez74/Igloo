package main

import (
	"encoding/gob"
	"igloo/cmd/models"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
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

	session := initSession()

	app := config{
		DB:       db,
		InfoLog:  infoLog,
		ErrorLog: errorLog,
		Session:  session,
	}

	err = http.ListenAndServe(os.Getenv("PORT"), app.routes())
	if err != nil {
		panic(err)
	}
}

func initDB() (*gorm.DB, error) {
	dsn := os.Getenv("DSN")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(
		&models.User{},
		&models.UserSettings{},
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

func initSession() *scs.SessionManager {
	gob.Register(models.User{})

	session := scs.New()

	session.Store = redisstore.New(initRedis())

	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true
	session.Cookie.Name = os.Getenv("COOKIE_NAME")
	session.Cookie.HttpOnly = true
	session.Cookie.Domain = os.Getenv("COOKIE_DOMAIN")
	session.Cookie.Path = "/"

	return session
}

func initRedis() *redis.Pool {
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", os.Getenv("REDIS"))
		},
	}

	return redisPool
}
