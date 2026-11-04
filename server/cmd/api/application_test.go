package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestSyncArtworkAfterScanAsksARunOfChangesForAPassOfChanges(t *testing.T) {
	artwork := &artworkRequests{}
	app := &Application{Artwork: artwork}

	app.syncArtworkAfterScan(true)
	if artwork.changes.Load() != 1 || artwork.count.Load() != 0 {
		t.Fatalf("after a run of changes: passes of changes = %d, full passes = %d; want 1 and 0", artwork.changes.Load(), artwork.count.Load())
	}

	app.syncArtworkAfterScan(false)
	if artwork.changes.Load() != 1 || artwork.count.Load() != 1 {
		t.Fatalf("after a library scan: passes of changes = %d, full passes = %d; want 1 and 1", artwork.changes.Load(), artwork.count.Load())
	}
}

func TestScanFinishedRefreshesPlannerStatisticsAndAsksForArtwork(t *testing.T) {
	app := newTestDBApp(t)
	artwork := &artworkRequests{}
	app.Artwork = artwork
	app.Wait = &sync.WaitGroup{}
	app.ScanContext = t.Context()

	_, err := app.DB.Exec("INSERT INTO albums (title, sort_title) VALUES ('Album', 'album')")
	if err != nil {
		t.Fatalf("seed an album: %v", err)
	}

	app.scanFinished(false)
	if artwork.count.Load() != 1 {
		t.Errorf("full artwork passes = %d, want 1", artwork.count.Load())
	}

	// The refresh runs on its own goroutine, tracked by app.Wait.
	app.Wait.Wait()
	var analyzed int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM sqlite_stat1 WHERE tbl = 'albums'").Scan(&analyzed)
	if err != nil {
		t.Fatalf("read albums statistics: %v", err)
	}
	if analyzed == 0 {
		t.Error("want albums analyzed after a scanner run")
	}
	if app.plannerStatsRunning || app.plannerStatsRerun {
		t.Errorf("after the refresh: running = %v, rerun = %v; want both false", app.plannerStatsRunning, app.plannerStatsRerun)
	}
}

// A run that finishes while a refresh is in progress asks for one more rather
// than starting a second refresh beside it, and the refresh then runs again.
func TestPlannerStatsRefreshRequestedWhileRunningRunsOnceMore(t *testing.T) {
	app := &Application{Wait: &sync.WaitGroup{}, ScanContext: t.Context()}
	started := make(chan struct{})
	release := make(chan struct{})
	var runs int
	app.plannerStatsRefresh = func(context.Context) error {
		runs++
		if runs == 1 {
			close(started)
			<-release
		}
		return nil
	}

	app.requestPlannerStatsRefresh()
	<-started
	// Two requests while the first refresh runs coalesce into one rerun.
	app.requestPlannerStatsRefresh()
	app.requestPlannerStatsRefresh()
	close(release)
	app.Wait.Wait()

	if runs != 2 {
		t.Errorf("refresh runs = %d, want 2", runs)
	}
	if app.plannerStatsRunning || app.plannerStatsRerun {
		t.Errorf("after the refreshes: running = %v, rerun = %v; want both false", app.plannerStatsRunning, app.plannerStatsRerun)
	}
}

// Shutdown cancels ScanContext, which interrupts a refresh: that is not an
// error worth logging, and it must not loop on a pending rerun.
func TestPlannerStatsRefreshCanceledAtShutdownIsQuiet(t *testing.T) {
	app := newTestDBApp(t)
	var logs bytes.Buffer
	app.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	app.Wait = &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(t.Context())
	app.ScanContext = ctx
	cancel()

	app.requestPlannerStatsRefresh()
	app.Wait.Wait()

	if strings.Contains(logs.String(), "failed to refresh query planner statistics") {
		t.Errorf("canceled refresh logged an error: %s", logs.String())
	}
	if app.plannerStatsRunning {
		t.Error("want the refresh finished after cancellation")
	}
}
