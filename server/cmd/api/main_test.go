package main

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/scanner/music"
	"igloo/cmd/internal/scanner/show"
)

type musicStartFunc func() scanner.StartResult

func (f musicStartFunc) Start() scanner.StartResult { return f() }

func (musicStartFunc) Status() music.Status {
	return music.Status{Progress: scanner.Progress{State: scanner.StateIdle, Phase: scanner.PhaseIdle, ActiveFiles: []string{}, Issues: []scanner.Issue{}}}
}

type movieStartFunc func() scanner.StartResult

func (f movieStartFunc) Start() scanner.StartResult { return f() }

type showStartFunc func() scanner.StartResult

func (f showStartFunc) Start() scanner.StartResult { return f() }

func (showStartFunc) Status() show.Status {
	return show.Status{Progress: scanner.Progress{State: scanner.StateIdle, Phase: scanner.PhaseIdle, ActiveFiles: []string{}, Issues: []scanner.Issue{}}}
}

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

	// The scanners captured the harness logger at construction; rebuild them
	// so the music scan reports through the recording logger.
	logger := &startupLogger{}
	app.Logger = logger
	app.initScanners()
	current := *app.CurrentSettings()
	current.MusicDir = sql.NullString{String: t.TempDir(), Valid: true}
	app.SetSettings(&current)
	app.MovieScanner = movieStartFunc(func() scanner.StartResult {
		logger.add("movie")
		return scanner.StartResult{Status: scanner.StartStarted}
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
		result    scanner.StartResult
		wantEvent string
	}{
		{
			name:      "not configured",
			result:    scanner.StartResult{Status: scanner.StartNotConfigured},
			wantEvent: "skipping movie library scan: movies directory is not configured",
		},
		{
			name:      "already running",
			result:    scanner.StartResult{Status: scanner.StartAlreadyRunning},
			wantEvent: "movie library scan is already in progress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)

			logger := &startupLogger{}
			app.Logger = logger
			startCalls := 0
			app.MovieScanner = movieStartFunc(func() scanner.StartResult {
				startCalls++
				return tc.result
			})

			app.startScanAtStartup(movieLibrary, app.MovieScanner.Start())

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
		result    scanner.StartResult
		wantEvent string
	}{
		{
			name:      "not configured",
			result:    scanner.StartResult{Status: scanner.StartNotConfigured},
			wantEvent: "skipping music library scan: music directory is not configured",
		},
		{
			name:      "already running",
			result:    scanner.StartResult{Status: scanner.StartAlreadyRunning},
			wantEvent: "music library scan is already in progress",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)

			logger := &startupLogger{}
			app.Logger = logger
			app.MovieScanner = movieStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartStarted} })
			startCalls := 0
			app.MusicScanner = musicStartFunc(func() scanner.StartResult {
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

func TestStartupStartsShowScanner(t *testing.T) {
	for _, status := range []scanner.StartStatus{scanner.StartStarted, scanner.StartNotConfigured, scanner.StartAlreadyRunning} {
		t.Run(fmt.Sprintf("%v", status), func(t *testing.T) {
			app := setupTestApp(t)
			calls := 0
			app.ShowScanner = showStartFunc(func() scanner.StartResult { calls++; return scanner.StartResult{Status: status} })
			app.MovieScanner = movieStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartStarted} })
			app.MusicScanner = musicStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartStarted} })
			startLibraryScansAtStartup(app)
			if calls != 1 {
				t.Fatal("startup did not invoke TV scanner")
			}
		})
	}
}
