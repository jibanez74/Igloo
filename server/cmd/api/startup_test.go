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
	} else if msg == "skipping movie library scan: movies directory is not configured" {
		l.add(msg)
	}
}
func (l *startupLogger) Warn(msg string, _ ...any) {
	if msg == "movie library scan is already in progress" {
		l.add(msg)
	}
}
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

func TestStartMovieScanAtStartupHandlesNonStartedResults(t *testing.T) {
	tests := []struct {
		name      string
		result    movie.StartResult
		wantEvent string
	}{
		{
			name:      "not configured",
			result:    movie.StartResult{Status: movie.StartNotConfigured},
			wantEvent: "skipping movie library scan: movies directory is not configured",
		},
		{
			name:      "already running",
			result:    movie.StartResult{Status: movie.StartAlreadyRunning},
			wantEvent: "movie library scan is already in progress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()

			logger := &startupLogger{}
			app.Logger = logger
			startCalls := 0
			app.MovieScanner = movieStartFunc(func() movie.StartResult {
				startCalls++
				return tc.result
			})

			startMovieScanAtStartup(app)

			if startCalls != 1 {
				t.Fatalf("movie scanner Start calls = %d, want 1", startCalls)
			}
			if len(logger.events) != 1 || logger.events[0] != tc.wantEvent {
				t.Fatalf("startup events = %v, want [%q]", logger.events, tc.wantEvent)
			}
		})
	}
}
