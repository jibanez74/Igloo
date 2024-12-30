package main

import (
	"embed"
	"igloo/cmd/database"
	"igloo/cmd/repository"
	"igloo/cmd/tmdb"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/redis"
)

//go:embed bin/*
var binaries embed.FS

type config struct {
	debug        bool
	port         string
	workDir      string
	repo         repository.Repo
	tmdb         tmdb.Tmdb
	wg           *sync.WaitGroup
	cookieSecret string
	cookieDomain string
	session      *session.Store
	ffmpeg       string
	ffprobe      string
	TranscodeDir string
}

func main() {
	var app config

	debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		panic(err)
	}
	app.debug = debug

	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	app.workDir = workDir

	app.port = os.Getenv("PORT")
	if app.port == "" {
		app.port = ":8080"
	}

	app.cookieSecret = os.Getenv("COOKIE_SECRET")
	if app.cookieSecret == "" {
		panic("cookie secret is required")
	}

	app.cookieDomain = os.Getenv("COOKIE_DOMAIN")
	if app.cookieDomain == "" {
		panic("cookie domain is required")
	}

	tmdbKey := os.Getenv("TMDB_API_KEY")
	if tmdbKey == "" {
		panic("tmdb key is required")
	}
	app.tmdb = tmdb.New(tmdbKey)

	db, err := database.New()
	if err != nil {
		panic(err)
	}
	app.repo = repository.New(db)

	ffmpegPath, err := app.extractBinary("ffmpeg")
	if err != nil {
		panic(err)
	}
	app.ffmpeg = ffmpegPath

	ffprobePath, err := app.extractBinary("ffprobe")
	if err != nil {
		panic(err)
	}
	app.ffprobe = ffprobePath

	app.initSession()

	app.wg = &sync.WaitGroup{}

	// this is a temp dir for testing hls transcoding
	app.transcodeDir = "/home/romany/videos"

	app.run()

	defer os.RemoveAll(filepath.Dir(ffmpegPath))
}

func (app *config) initSession() {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 6379 // Default Redis port
	username := os.Getenv("REDIS_USER")
	password := os.Getenv("REDIS_PASSWORD")

	storage := redis.New(redis.Config{
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		Database:  0,
		Reset:     false,
		PoolSize:  10,
		TLSConfig: nil,
	})

	app.session = session.New(session.Config{
		Storage:        storage,
		Expiration:     24 * time.Hour,
		KeyLookup:      "cookie:session_id",
		CookieDomain:   app.cookieDomain,
		CookiePath:     "/",
		CookieSecure:   !app.debug, // True in production
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
	})
}

func (app *config) extractBinary(name string) (string, error) {
	data, err := binaries.ReadFile("bin/" + name)
	if err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "igloo")
	if err != nil {
		return "", err
	}

	binaryPath := filepath.Join(tempDir, name)

	err = os.WriteFile(binaryPath, data, 0755)
	if err != nil {
		return "", err
	}

	return binaryPath, nil
}
