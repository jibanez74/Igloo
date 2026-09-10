package movie

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

// Workers must not touch the database: the pool is pinned to one connection,
// so a worker read would wait behind the coordinator's open transaction before
// it could even start ffprobe. A closed database fails every query, so a
// successful prepare proves the worker stayed off it.
func TestWorkersPrepareWithoutDatabase(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	details := retryMovieFixture(t)
	s.tmdb = &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Local"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}
	path := filepath.Join(t.TempDir(), "Local (2001).mkv")
	err := os.WriteFile(path, []byte("movie"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	fixture.db.Close()
	_, err = s.queries.GetMovieByPath(context.Background(), path)
	if err == nil {
		t.Fatal("closed database still answers queries")
	}

	file := scanner.ScanFile{Path: path, Ext: "mkv"}
	probed := s.prepareFile(context.Background(), probeJob{file: file})
	if probed.err != nil || probed.resolved == nil || probed.resolved.params.Title != "Local" {
		t.Fatalf("probe used the database: err=%v resolved=%+v", probed.err, probed.resolved)
	}
	probed.inspection.Close()

	inspection, err := scanner.InspectFileMetadata(context.Background(), path, nil, s.now)
	if err != nil {
		t.Fatal(err)
	}
	inspection.Close()
	baseline := movieScanEntry{FileFingerprint: inspection.Fingerprint, ID: 1, FilePath: path, HasFingerprint: true}
	enriched := s.prepareEnrichment(context.Background(), enrichmentJob{file: file, baseline: baseline})
	if enriched.err != nil || enriched.resolved == nil || enriched.resolved.tmdbMovie == nil || enriched.resolved.tmdbMovie.TmdbID != 42 {
		t.Fatalf("enrichment used the database: err=%v resolved=%+v", enriched.err, enriched.resolved)
	}
	enriched.inspection.Close()
}

func TestEnrichmentEligibilityBacksOffAfterRepeatedMisses(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	missedAt := helpers.NullInt64(now.Add(-time.Hour).Unix())
	cases := []struct {
		name  string
		entry movieScanEntry
		at    time.Time
		want  bool
	}{
		{"identified and not re-queued", movieScanEntry{TmdbID: helpers.NullInt64(7)}, now, false},
		{"never attempted", movieScanEntry{}, now, true},
		{"first miss retries next scan", movieScanEntry{PendingRetry: true, RetryAttempts: 1, LastAttemptAt: missedAt}, now, true},
		{"second miss waits a day", movieScanEntry{PendingRetry: true, RetryAttempts: 2, LastAttemptAt: missedAt}, now, false},
		{"second miss eligible after a day", movieScanEntry{PendingRetry: true, RetryAttempts: 2, LastAttemptAt: missedAt}, now.Add(24 * time.Hour), true},
		{"third miss waits two days", movieScanEntry{PendingRetry: true, RetryAttempts: 3, LastAttemptAt: missedAt}, now.Add(24 * time.Hour), false},
		{"backoff caps at a week", movieScanEntry{PendingRetry: true, RetryAttempts: 40, LastAttemptAt: missedAt}, now.Add(7 * 24 * time.Hour), true},
		{"re-queued by rescan resets", movieScanEntry{TmdbID: helpers.NullInt64(7), PendingRetry: true}, now, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.entry.enrichmentEligible(tc.at)
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
	defer fixture.db.Close()
	s := fixture.scanner
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	client := &stubMovieScannerTmdb{searchErr: tmdb.ErrNoMoviesFound}
	s.tmdb = client
	root := t.TempDir()
	path := filepath.Join(root, "Unknown (2001).mkv")
	err := os.WriteFile(path, []byte("movie"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Now().Add(2 * time.Minute)
	s.now = func() time.Time { return clock }

	searches := func() int { return len(client.searchCalls) }
	s.scan(root)
	first := s.Status()
	if first.State != StateCompletedWithIssues || first.EnrichmentUnmatched != 1 || first.PendingEnrichment != 1 || searches() == 0 {
		t.Fatalf("first miss: %+v searches=%d", first, searches())
	}
	var attempts, lastAttempt sql.NullInt64
	err = s.db.QueryRow("SELECT attempts, last_attempt_at FROM movie_tmdb_retries").Scan(&attempts, &lastAttempt)
	if err != nil || attempts.Int64 != 1 || lastAttempt.Int64 != clock.Unix() {
		t.Fatalf("miss bookkeeping: attempts=%v last=%v err=%v", attempts, lastAttempt, err)
	}

	before := searches()
	s.scan(root)
	second := s.Status()
	if second.State != StateCompletedWithIssues || second.EnrichmentUnmatched != 1 || searches() == before {
		t.Fatalf("second scan skipped the free retry: %+v", second)
	}

	before = searches()
	s.scan(root)
	third := s.Status()
	if third.State != StateCompleted || third.EnrichmentTotal != 0 || third.PendingEnrichment != 1 || third.IssueCount != 0 || searches() != before {
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
func TestScanWithoutTmdbCompletesCleanly(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	root := t.TempDir()
	err := os.WriteFile(filepath.Join(root, "Local (2001).mkv"), []byte("movie"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	s.scan(root)
	status := s.Status()
	if status.State != StateCompleted || status.Imported != 1 || status.PendingEnrichment != 1 || status.EnrichmentTotal != 1 {
		t.Fatalf("scan without TMDB: %+v", status)
	}
}
