package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/movie"
)

// inotifyInstances counts this process's open inotify instances: the watcher
// holds exactly one while it runs.
func inotifyInstances(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("list descriptors: %v", err)
	}
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && strings.Contains(target, "inotify") {
			count++
		}
	}
	return count
}

// waitForInotifyInstances waits for the session goroutine to open its
// instance, which it does after Sync returns.
func waitForInotifyInstances(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for inotifyInstances(t) != want {
		if time.Now().After(deadline) {
			t.Fatalf("inotify instances = %d, want %d", inotifyInstances(t), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestUpdateGeneralSettings_StartsAndStopsTheWatcher(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	staticDir := filepath.Join(t.TempDir(), "static")
	before := inotifyInstances(t)

	w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBodyWithWatcher(staticDir, true))
	if w.Code != http.StatusOK {
		t.Fatalf("enabling: status %d: %s", w.Code, w.Body.String())
	}
	waitForInotifyInstances(t, before+1)

	// Off means off: the instance is closed before the response is written.
	w = serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", generalSettingsBodyWithWatcher(staticDir, false))
	if w.Code != http.StatusOK || inotifyInstances(t) != before {
		t.Fatalf("disabling: status %d, inotify instances %d, want %d", w.Code, inotifyInstances(t), before)
	}
}

// scannerFake is a movie scanner with separate library and change starts.
type scannerFake struct {
	start   func() scanner.StartResult
	changes func(scanner.Changes) scanner.StartResult
}

func (f scannerFake) Start() scanner.StartResult { return f.start() }

func (f scannerFake) StartChanges(changes scanner.Changes) scanner.StartResult {
	return f.changes(changes)
}

func (scannerFake) Status() movie.Status { return movie.Status{Progress: idleScanProgress()} }

// changesRecorder records the runs of watcher-detected changes a library got.
type changesRecorder struct {
	mu   sync.Mutex
	runs []string
}

func (r *changesRecorder) record(library string, changes scanner.Changes) scanner.StartResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := "changes"
	if changes.CatchUp {
		kind = "catch-up"
	}
	r.runs = append(r.runs, library+" "+kind)
	return scanner.StartResult{Status: scanner.StartStarted}
}

func TestStartLibraryWorkFollowsTheStoredWatcherSetting(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("watcher %t", enabled), func(t *testing.T) {
			app := setupTestApp(t)
			before := inotifyInstances(t)
			root := t.TempDir()
			settings := *app.CurrentSettings()
			settings.EnableWatcher = enabled
			settings.MoviesDir = sql.NullString{String: root, Valid: true}
			app.SetSettings(&settings)
			var started []string
			recorder := &changesRecorder{}
			app.MovieScanner = scannerFake{start: func() scanner.StartResult {
				started = append(started, "movie")
				return scanner.StartResult{Status: scanner.StartStarted}
			}, changes: func(c scanner.Changes) scanner.StartResult { return recorder.record("movie", c) }}
			app.ShowScanner = showStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartNotConfigured} })
			app.MusicScanner = musicStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartNotConfigured} })

			startLibraryWork(app)

			if !enabled && inotifyInstances(t) != before {
				t.Fatal("with the watcher off nothing may watch the libraries")
			}
			if enabled {
				waitForInotifyInstances(t, before+1)
			}
			if !enabled {
				if !slices.Equal(started, []string{"movie"}) {
					t.Fatalf("with the watcher off the startup scans must run, got %q", started)
				}
				return
			}
			if len(started) != 0 {
				t.Fatalf("with the watcher on its catch-up replaces the startup scans, got %q", started)
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				recorder.mu.Lock()
				runs := slices.Clone(recorder.runs)
				recorder.mu.Unlock()
				if slices.Contains(runs, "movie catch-up") {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("the watcher never walked the movie library: %q", runs)
				}
				time.Sleep(10 * time.Millisecond)
			}
			app.Watcher.Sync(false)
		})
	}
}
