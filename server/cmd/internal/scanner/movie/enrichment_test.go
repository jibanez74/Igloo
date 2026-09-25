package movie

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/tmdb"
)

// The backoff arithmetic is covered in the scanner package; these cases pin
// the pending-state gate in front of it.
func TestEnrichmentEligibilityRequiresPendingState(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		entry movieScanEntry
		want  bool
	}{
		{"identified and not re-queued", movieScanEntry{TmdbID: helpers.NullInt64(7)}, false},
		{"never attempted", movieScanEntry{}, true},
		{"re-queued by rescan resets", movieScanEntry{TmdbID: helpers.NullInt64(7), PendingRetry: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.entry.enrichmentEligible(now)
			if got != tc.want {
				t.Fatalf("eligible=%v want=%v", got, tc.want)
			}
		})
	}
}

// Repeated misses must stop costing a TMDB search per scan, and a run whose
// only outstanding work is backed-off enrichment completes without issues.
func TestRepeatedMissesBackOffAcrossScans(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("3600")}
	client := &stubMovieScannerTmdb{searchErr: tmdb.ErrNoMoviesFound}
	s.tmdb = client
	root := t.TempDir()
	path := filepath.Join(root, "Unknown (2001).mkv")
	scannertest.WriteFile(t, path, "movie")
	clock := scannertest.SettledNow()
	s.now = func() time.Time { return clock }

	searches := func() int { return len(client.searchCalls) }
	s.scan(root)
	first := s.Status()
	if first.State != scanner.StateCompletedWithIssues || first.EnrichmentUnmatched != 1 || first.PendingEnrichment != 1 || searches() == 0 {
		t.Fatalf("first miss: %+v searches=%d", first, searches())
	}
	var attempts, lastAttempt sql.NullInt64
	err := s.tx.DB.QueryRow("SELECT attempts, last_attempt_at FROM movie_tmdb_retries").Scan(&attempts, &lastAttempt)
	if err != nil || attempts.Int64 != 1 || lastAttempt.Int64 != clock.Unix() {
		t.Fatalf("miss bookkeeping: attempts=%v last=%v err=%v", attempts, lastAttempt, err)
	}

	before := searches()
	s.scan(root)
	second := s.Status()
	if second.State != scanner.StateCompletedWithIssues || second.EnrichmentUnmatched != 1 || searches() == before {
		t.Fatalf("second scan skipped the free retry: %+v", second)
	}

	before = searches()
	s.scan(root)
	third := s.Status()
	if third.State != scanner.StateCompleted || third.EnrichmentTotal != 0 || third.PendingEnrichment != 1 || third.IssueCount != 0 || searches() != before {
		t.Fatalf("third scan did not back off: %+v searches=%d", third, searches()-before)
	}

	clock = clock.Add(25 * time.Hour)
	before = searches()
	s.scan(root)
	fourth := s.Status()
	if fourth.EnrichmentTotal != 1 || searches() == before {
		t.Fatalf("backoff never expired: %+v", fourth)
	}
}

// Without a TMDB client every movie stays pending, which is not an issue.
// EnrichmentTotal stays 0 so the UI shows no enrichment progress at all rather
// than a bar frozen at 0/N.
func TestScanWithoutTmdbCompletesCleanly(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("3600")}
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "Local (2001).mkv"), "movie")
	s.scan(root)
	status := s.Status()
	if status.State != scanner.StateCompleted || status.Imported != 1 || status.PendingEnrichment != 1 || status.EnrichmentTotal != 0 {
		t.Fatalf("scan without TMDB: %+v", status)
	}
}

// A file that changed or is still settling is deferred, not failed: enrichment
// stays pending and retries on a later scan, so the user sees no issue for it.
func TestEnrichmentDeferralIsNotAFailure(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	report := newScanReport(Status{})
	result := enrichmentResult{
		job: enrichmentJob{file: scanner.ScanFile{Path: "/library/Local (2001).mkv"}},
		err: &scanner.FileDeferral{Reason: scanner.FileChanged},
	}

	s.recordEnrichment(report, result, enrichmentSkipped)

	if report.status.EnrichmentFailed != 0 || report.status.EnrichmentUnmatched != 0 || report.IssueCount() != 0 {
		t.Fatalf("deferral recorded as a failure: %+v issues=%d", report.status, report.IssueCount())
	}
}

// A file with no title at all has nothing to search, so enrichment records a
// miss without asking TMDB for an empty query. Release-noise-only names keep
// their raw title and are searched like any other.
func TestEnrichmentRecordsMissForFilenameWithoutTitle(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	client := &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Noise"}}, detailMovies: map[int]tmdb.TmdbMovie{42: retryMovieFixture(t)}}
	s.tmdb = client
	path := filepath.Join(t.TempDir(), ".mkv")
	scannertest.WriteFile(t, path, "movie")
	if movieTitleYear(path).Title != "" {
		t.Fatalf("fixture has a title: %+v", movieTitleYear(path))
	}
	report := s.processBatchReport(context.Background(), nextMovieScan(t, s), []scanner.ScanFile{{Path: path, Ext: "mkv"}})
	if report.status.Imported != 1 || report.status.EnrichmentUnmatched != 1 || report.status.EnrichmentFailed != 0 {
		t.Fatalf("untitled file: %+v", report.status)
	}
	if len(client.searchCalls) != 0 || len(client.detailCalls) != 0 {
		t.Fatalf("TMDB was queried for an empty title: searches=%+v details=%+v", client.searchCalls, client.detailCalls)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_tmdb_retries") != 1 {
		t.Fatal("miss was not recorded for a later retry")
	}
}
