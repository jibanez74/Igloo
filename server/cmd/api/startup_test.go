package main

import (
	"database/sql"
	"sync"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/scanner/music"
)

type musicStartFunc func() music.StartResult

func (f musicStartFunc) Start() music.StartResult { return f() }

func (musicStartFunc) Status() music.Status {
	return music.Status{Progress: scanner.Progress{State: scanner.StateIdle, Phase: scanner.PhaseIdle, ActiveFiles: []string{}, Issues: []scanner.Issue{}}}
}

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
	if msg == "music scan phase" {
		l.add("music")
	} else if msg == "skipping movie library scan: movies directory is not configured" || msg == "skipping music library scan: music directory is not configured" {
		l.add(msg)
	}
}
func (l *startupLogger) Warn(msg string, _ ...any) {
	if msg == "movie library scan is already in progress" || msg == "music library scan is already in progress" {
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
	app.MusicScanner = music.New(music.Dependencies{
		DB: app.DB, Queries: app.Queries, Logger: app.Logger, Wait: app.Wait,
		ScannerDBMu:           &app.ScannerDBMu,
		CurrentMusicDirectory: func() sql.NullString { return app.CurrentSettings().MusicDir },
	})
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

func TestStartMusicScanAtStartupHandlesNonStartedResults(t *testing.T) {
	tests := []struct {
		name      string
		result    music.StartResult
		wantEvent string
	}{
		{
			name:      "not configured",
			result:    music.StartResult{Status: music.StartNotConfigured},
			wantEvent: "skipping music library scan: music directory is not configured",
		},
		{
			name:      "already running",
			result:    music.StartResult{Status: music.StartAlreadyRunning},
			wantEvent: "music library scan is already in progress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()

			logger := &startupLogger{}
			app.Logger = logger
			app.MovieScanner = movieStartFunc(func() movie.StartResult { return movie.StartResult{Status: movie.StartStarted} })
			startCalls := 0
			app.MusicScanner = musicStartFunc(func() music.StartResult {
				startCalls++
				return tc.result
			})

			startLibraryScansAtStartup(app)

			if startCalls != 1 {
				t.Fatalf("music scanner Start calls = %d, want 1", startCalls)
			}
			if len(logger.events) != 1 || logger.events[0] != tc.wantEvent {
				t.Fatalf("startup events = %v, want [%q]", logger.events, tc.wantEvent)
			}
		})
	}
}

func (movieStartFunc) Status() movie.Status {
	return movie.Status{Progress: scanner.Progress{State: scanner.StateIdle, Phase: scanner.PhaseIdle, ActiveFiles: []string{}, Issues: []scanner.Issue{}}}
}
