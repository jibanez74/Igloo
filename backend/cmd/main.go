package main

import (
	"embed"
	"errors"
	"fmt"
	"igloo/cmd/database"
	"igloo/cmd/repository"
	"igloo/cmd/tmdb"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

//go:embed bin/*
var binaries embed.FS

type config struct {
	debug        bool
	workDir      string
	repo         repository.Repo
	tmdb         tmdb.Tmdb
	wg           *sync.WaitGroup
	cookieSecret string
	cookieDomain string
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

	app.wg = &sync.WaitGroup{}

	go app.listenForShutdown()

	app.serve()

	defer os.RemoveAll(filepath.Dir(ffmpegPath))
}

func (app *config) serve() {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	srv := &http.Server{
		Addr:    port,
		Handler: app.routes(),
	}

	log.Println("Starting web server...")

	err := srv.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func (app *config) listenForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.shutdown()
	os.Exit(0)
}

func (app *config) shutdown() {
	log.Println("would run cleanup tasks...")
	app.wg.Wait()
	log.Println("closing channels and shutting down application...")
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
