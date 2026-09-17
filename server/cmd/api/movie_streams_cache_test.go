package main

import (
	"context"
	"testing"
)

// The watch-room pin checks run once per byte-range request and once per
// manifest refresh, so the second call must not reach SQLite. Rewriting the row
// behind the cache is how the test observes that: a cached read still reports
// the old stream index.
func TestMovieStreamsAreServedFromCacheUntilInvalidated(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	movieID := insertTestHLSMovieFixture(t, app, "h264", 720)

	first, err := app.movieStreamsFor(context.Background(), movieID)
	if err != nil {
		t.Fatalf("resolve movie streams: %v", err)
	}
	if len(first.Audio) == 0 {
		t.Fatal("fixture has no audio streams; the test cannot observe a drift")
	}
	original := first.Audio[0].StreamIndex

	_, err = app.DB.Exec(`UPDATE audio_streams SET stream_index = stream_index + 10 WHERE movie_id = ?`, movieID)
	if err != nil {
		t.Fatalf("shift audio stream index: %v", err)
	}

	cached, err := app.movieStreamsFor(context.Background(), movieID)
	if err != nil {
		t.Fatalf("resolve cached movie streams: %v", err)
	}
	if cached.Audio[0].StreamIndex != original {
		t.Errorf("second lookup re-queried the database: stream_index = %d, want the cached %d",
			cached.Audio[0].StreamIndex, original)
	}

	// A rescan commits through this, which is what lets the pin checks keep
	// detecting stream drift.
	app.invalidateCommittedMovie(movieID)

	fresh, err := app.movieStreamsFor(context.Background(), movieID)
	if err != nil {
		t.Fatalf("resolve movie streams after invalidation: %v", err)
	}
	if fresh.Audio[0].StreamIndex != original+10 {
		t.Errorf("stream_index after invalidation = %d, want %d; the rescan's eviction did not take",
			fresh.Audio[0].StreamIndex, original+10)
	}
}
