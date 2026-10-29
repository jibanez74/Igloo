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
