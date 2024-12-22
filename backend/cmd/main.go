package main

import (
	"embed"
	"errors"
	"fmt"
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
}

func main() {
	var app config

	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	app.workDir = workDir

	debug, err := strconv.ParseBool(os.Getenv("DEBUG"))
	if err != nil {
		panic(err)
	}
	app.debug = debug

	db, err := database.New()
	if err != nil {
		panic(err)
	}
	app.repo = repository.New(db)

	ffmpegPath, ffprobePath, err := extractBinaries()
	if err != nil {
		panic(fmt.Errorf("failed to extract binaries: %w", err))
	}
	app.ffmpeg = ffmpegPath
	app.ffprobe = ffprobePath

	tmdbKey := os.Getenv("TMDB_API_KEY")
	if tmdbKey == "" {
		panic(errors.New("TMDB_API_KEY is not set"))
	}
	app.tmdb = tmdb.New(tmdbKey)

	secret := os.Getenv("COOKIE_SECRET")
	if secret == "" {
		panic(errors.New("COOKIE_SECRET is not set"))
	}
	app.cookieSecret = secret

	cookieDomain := os.Getenv("COOKIE_DOMAIN")
	if cookieDomain == "" {
		cookieDomain = "localhost"
	}
	app.cookieDomain = cookieDomain

	dirs := []string{
		"static",
		"static/movies",
		"static/movies/thumb",
		"static/movies/art",
		"static/artists",
		"static/studios",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			panic(fmt.Errorf("failed to create directory %s: %w", dir, err))
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}
	app.port = port

	app.initSession()

	app.wg = &sync.WaitGroup{}

	err = app.run()
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(filepath.Dir(ffmpegPath))
}

func (app *config) initSession() {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	port, err := strconv.Atoi(os.Getenv("REDIS_PORT"))
	if err != nil {
		port = 6379 // Default Redis port
	}

	storage := redis.New(redis.Config{
		Host:      host,
		Port:      port,
		Username:  "",
		Password:  "",
		Database:  0,
		Reset:     false,
		PoolSize:  10,
		TLSConfig: nil,
	})

	app.session = session.New(session.Config{
		Storage:        storage,
		Expiration:     24 * time.Hour,
		CookieDomain:   app.cookieDomain,
		CookiePath:     "/",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: "strict",
	})
}

func extractBinaries() (string, string, error) {
	tempDir, err := os.MkdirTemp("", "igloo-binaries")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	ffmpegPath := filepath.Join(tempDir, "ffmpeg")
	ffmpegBin, err := binaries.ReadFile("bin/ffmpeg")
	if err != nil {
		return "", "", fmt.Errorf("failed to read ffmpeg binary: %w", err)
	}

	if err := os.WriteFile(ffmpegPath, ffmpegBin, 0755); err != nil {
		return "", "", fmt.Errorf("failed to write ffmpeg binary: %w", err)
	}

	ffprobePath := filepath.Join(tempDir, "ffprobe")
	ffprobeBin, err := binaries.ReadFile("bin/ffprobe")
	if err != nil {
		return "", "", fmt.Errorf("failed to read ffprobe binary: %w", err)
	}

	if err := os.WriteFile(ffprobePath, ffprobeBin, 0755); err != nil {
		return "", "", fmt.Errorf("failed to write ffprobe binary: %w", err)
	}

	return ffmpegPath, ffprobePath, nil
}
