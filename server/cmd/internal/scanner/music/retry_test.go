package music

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner/scannertest"
)

// A compound credit that stayed combined offline and later missed on Spotify
// is split by the reconciliation pass that runs after the retry loops, even
// when no retry candidate is left to trigger it. A canceled context stops
// before touching credits, and a closed database surfaces the query error.
func TestReconcilePendingCompoundCreditsSplitsCommittedMisses(t *testing.T) {
	s := setupMusicScanner(t)
	ctx := context.Background()
	scanTaggedTrack(t, s, newMusicScanContext(nil), filepath.Join(t.TempDir(), "one"), 1, ffprobe.FormatTags{Title: "One", Artist: "One & Two"})
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians") != 1 {
		t.Fatal("offline scan split the compound credit early")
	}
	_, err := s.tx.DB.Exec("INSERT INTO music_spotify_matches(entity_type, entity_id, status, reason) SELECT 'musician', id, 'unmatched', 'no_results' FROM musicians")
	if err != nil {
		t.Fatal(err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	err = s.reconcilePendingCompoundCredits(canceled, newMusicScanContext(nil))
	if !errors.Is(err, context.Canceled) || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM musicians") != 1 {
		t.Fatalf("canceled reconciliation: err=%v", err)
	}

	err = s.reconcilePendingCompoundCredits(ctx, newMusicScanContext(nil))
	if err != nil {
		t.Fatal(err)
	}
	split := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id = tm.musician_id WHERE m.name IN ('One', 'Two')")
	combined := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM track_musicians tm JOIN musicians m ON m.id = tm.musician_id WHERE m.name = 'One & Two'")
	if split != 2 || combined != 0 {
		t.Fatalf("credits after reconciliation: split=%d combined=%d", split, combined)
	}

	err = s.tx.DB.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = s.reconcilePendingCompoundCredits(ctx, newMusicScanContext(nil))
	if err == nil {
		t.Fatal("closed database did not fail the candidate query")
	}
}
