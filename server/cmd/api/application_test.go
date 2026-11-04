package main

import "testing"

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

	_, err := app.DB.Exec("INSERT INTO albums (title, sort_title) VALUES ('Album', 'album')")
	if err != nil {
		t.Fatalf("seed an album: %v", err)
	}

	app.scanFinished(false)

	var analyzed int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM sqlite_stat1 WHERE tbl = 'albums'").Scan(&analyzed)
	if err != nil {
		t.Fatalf("read albums statistics: %v", err)
	}
	if analyzed == 0 {
		t.Error("want albums analyzed after a scanner run")
	}
	if artwork.count.Load() != 1 {
		t.Errorf("full artwork passes = %d, want 1", artwork.count.Load())
	}
}
