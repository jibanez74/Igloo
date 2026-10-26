package main

import (
	"slices"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/scanner/music"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/scanner/show"
)

// startupFake records which kind of run startup asks a scanner for.
type startupFake[S any] struct {
	run func(kind string) scanner.StartResult
}

func (f startupFake[S]) Start() scanner.StartResult { return f.run("library scan") }

func (f startupFake[S]) StartChanges(changes scanner.Changes) scanner.StartResult {
	kind := "changes"
	guarded := changes.Guarded()
	if guarded {
		kind = "catch-up"
	}
	return f.run(kind)
}

func (startupFake[S]) Status() S {
	var status S
	return status
}

// Startup walks every library as a catch-up, whose cleanup keeps the files of
// a disk that has not mounted yet, rather than as a library scan.
func TestStartLibraryScansAtStartupStartsMovieShowThenMusic(t *testing.T) {
	app := setupTestApp(t)

	var order []string
	started := func(library string) func(string) scanner.StartResult {
		return func(kind string) scanner.StartResult {
			order = append(order, library+" "+kind)
			return scanner.StartResult{Status: scanner.StartStarted}
		}
	}
	app.MovieScanner = startupFake[movie.Status]{run: started("movie")}
	app.ShowScanner = startupFake[show.Status]{run: started("show")}
	app.MusicScanner = startupFake[music.Status]{run: started("music")}

	startLibraryScansAtStartup(app)

	want := []string{"movie catch-up", "show catch-up", "music catch-up"}
	if !slices.Equal(order, want) {
		t.Fatalf("start order = %v, want %v", order, want)
	}
}

func TestStartLibraryScansAtStartupLogsScansThatDidNotStart(t *testing.T) {
	tests := []struct {
		library  string
		status   scanner.StartStatus
		wantInfo string
		wantWarn string
	}{
		{library: "movie", status: scanner.StartNotConfigured, wantInfo: "skipping movie library scan: movies directory is not configured"},
		{library: "movie", status: scanner.StartAlreadyRunning, wantWarn: "movie library scan is already in progress"},
		{library: "show", status: scanner.StartNotConfigured, wantInfo: "skipping show library scan: shows directory is not configured"},
		{library: "show", status: scanner.StartAlreadyRunning, wantWarn: "show library scan is already in progress"},
		{library: "music", status: scanner.StartNotConfigured, wantInfo: "skipping music library scan: music directory is not configured"},
		{library: "music", status: scanner.StartAlreadyRunning, wantWarn: "music library scan is already in progress"},
	}

	for _, tc := range tests {
		t.Run(tc.library+"/"+tc.wantInfo+tc.wantWarn, func(t *testing.T) {
			app := setupTestApp(t)
			logger := &scannertest.Logger{}
			app.Logger = logger

			result := func(library string) func() scanner.StartResult {
				return func() scanner.StartResult {
					if library == tc.library {
						return scanner.StartResult{Status: tc.status}
					}
					return scanner.StartResult{Status: scanner.StartStarted}
				}
			}
			app.MovieScanner = movieStartFunc(result("movie"))
			app.ShowScanner = showStartFunc(result("show"))
			app.MusicScanner = musicStartFunc(result("music"))

			startLibraryScansAtStartup(app)

			// Started scans log nothing here, so each list holds at most the
			// one entry this library's result produced.
			if got := entryMessages(logger.InfoEntries); !slices.Equal(got, nonEmpty(tc.wantInfo)) {
				t.Errorf("info entries = %q, want %q", got, nonEmpty(tc.wantInfo))
			}
			if got := entryMessages(logger.WarnEntries); !slices.Equal(got, nonEmpty(tc.wantWarn)) {
				t.Errorf("warn entries = %q, want %q", got, nonEmpty(tc.wantWarn))
			}
		})
	}
}

func entryMessages(entries []scannertest.LogEntry) []string {
	messages := []string{}
	for _, entry := range entries {
		messages = append(messages, entry.Msg)
	}
	return messages
}

func nonEmpty(message string) []string {
	if message == "" {
		return []string{}
	}
	return []string{message}
}
