package main

import (
	"database/sql"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/scanner/movie"
)

type movieStartFunc func() movie.StartResult

func (f movieStartFunc) Start() movie.StartResult { return f() }

type startupLogger struct {
	mu     sync.Mutex
	events []string
}

func (l *startupLogger) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *startupLogger) Debug(string, ...any) {}
func (l *startupLogger) Info(msg string, _ ...any) {
	if strings.HasPrefix(msg, "scanning music directory:") {
		l.add("music")
	}
}
func (l *startupLogger) Warn(string, ...any)  {}
func (l *startupLogger) Error(string, ...any) {}

func TestStartLibraryScansAtStartupStartsMovieBeforeMusic(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	logger := &startupLogger{}
	app.Logger = logger
	app.Wait = &sync.WaitGroup{}
	current := *app.CurrentSettings()
	current.MusicDir = sql.NullString{String: t.TempDir(), Valid: true}
	app.SetSettings(&current)
	app.MovieScanner = movieStartFunc(func() movie.StartResult {
		logger.add("movie")
		return movie.StartResult{Status: movie.StartStarted}
	})

	startLibraryScansAtStartup(app)
	app.Wait.Wait()

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if len(logger.events) < 2 || logger.events[0] != "movie" || logger.events[1] != "music" {
		t.Fatalf("startup scan order = %v, want movie then music", logger.events)
	}
}
