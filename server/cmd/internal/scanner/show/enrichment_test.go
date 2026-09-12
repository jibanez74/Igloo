package show

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
)

// The enrichment total is known before the first request, so the progress
// bar can move; every entity then reports an outcome.
func TestShowEnrichmentTotalPublishedBeforeWork(t *testing.T) {
	s, _, root := setupScanner(t)
	writeFile(t, root, "Example (2020)/Season 1/S01E01E02.mkv", "a")
	writeFile(t, root, "Example (2020)/Season 1/S01E03.mkv", "b")
	client := &testTMDB{}
	var seen Status
	client.showHook = func(id int) (*tmdb.TVShow, error) {
		seen = s.Status()
		return (&testTMDB{}).GetShowDetails(context.Background(), id)
	}
	s.Tmdb = client
	scanOK(t, s, root)
	if seen.EnrichmentTotal != 5 || seen.EnrichmentProcessed != 0 || seen.Phase != scanner.PhaseEnrichment {
		t.Fatalf("total was not published before the first request: %+v", seen)
	}
	status := s.Status()
	if status.EnrichmentTotal != 5 || status.EnrichmentProcessed != 5 || status.Enriched != 5 || status.EnrichmentUnmatched != 0 || status.Episodes != 3 || status.PendingEnrichment != 0 || status.State != scanner.StateCompleted {
		t.Fatalf("final counters: %+v", status)
	}
}

// Repeated misses must stop costing a TMDB search per scan, and a run whose
// only outstanding work is backed-off enrichment completes without issues.
func TestShowUnmatchedBacksOffAcrossScans(t *testing.T) {
	s, _, root := setupScanner(t)
	writeFile(t, root, "Unknown (2001)/Season 1/S01E01.mkv", "a")
	client := &testTMDB{}
	client.searchHook = func(string, int) ([]tmdb.TVShow, error) { return nil, tmdb.ErrNoShowsFound }
	s.Tmdb = client
	clock := time.Now().Add(2 * time.Minute)
	s.Now = func() time.Time { return clock }

	scanOK(t, s, root)
	first := s.Status()
	if first.State != scanner.StateCompletedWithIssues || first.EnrichmentUnmatched != 1 || first.EnrichmentTotal != 1 || first.EnrichmentProcessed != 1 || first.PendingEnrichment != 3 || client.searchCalls != 1 || client.showCalls != 0 {
		t.Fatalf("first miss: %+v searches=%d", first, client.searchCalls)
	}
	var attempts, lastAttempt sql.NullInt64
	err := s.DB.QueryRow("SELECT attempts, last_attempt_at FROM show_tmdb_retries").Scan(&attempts, &lastAttempt)
	if err != nil || attempts.Int64 != 1 || lastAttempt.Int64 != clock.Unix() {
		t.Fatalf("miss bookkeeping: attempts=%v last=%v err=%v", attempts, lastAttempt, err)
	}

	scanOK(t, s, root)
	second := s.Status()
	if second.State != scanner.StateCompletedWithIssues || second.EnrichmentUnmatched != 1 || client.searchCalls != 2 {
		t.Fatalf("second scan skipped the free retry: %+v", second)
	}

	scanOK(t, s, root)
	third := s.Status()
	if third.State != scanner.StateCompleted || third.EnrichmentTotal != 0 || third.PendingEnrichment != 3 || third.IssueCount != 0 || client.searchCalls != 2 {
		t.Fatalf("third scan did not back off: %+v searches=%d", third, client.searchCalls)
	}

	clock = clock.Add(25 * time.Hour)
	scanOK(t, s, root)
	fourth := s.Status()
	if fourth.EnrichmentTotal != 1 || client.searchCalls != 3 {
		t.Fatalf("backoff never expired: %+v", fourth)
	}
}

// Provider failures stop dispatch for the run: an authentication failure at
// once, transient failures after three in a row. A definitive no-match is
// not a provider failure.
func TestShowEnrichmentCircuitBreaker(t *testing.T) {
	// Five shows carry three entities each; a show that cannot be resolved
	// drops its season and episode from the total.
	for _, tc := range []struct {
		name                               string
		err                                error
		searches, failed, unmatched, total int
		stopped                            bool
	}{
		{"authentication", &tmdb.StatusError{StatusCode: http.StatusUnauthorized}, 1, 1, 0, 13, true},
		{"transient", &tmdb.StatusError{StatusCode: http.StatusServiceUnavailable}, 3, 3, 0, 9, true},
		{"timeout", context.DeadlineExceeded, 3, 3, 0, 9, true},
		{"no match", tmdb.ErrNoShowsFound, 5, 0, 5, 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			for _, show := range []string{"A", "B", "C", "D", "E"} {
				writeFile(t, root, show+"/Season 1/S01E01.mkv", show)
			}
			client := &testTMDB{}
			client.searchHook = func(string, int) ([]tmdb.TVShow, error) { return nil, tc.err }
			s.Tmdb = client
			scanOK(t, s, root)
			status := s.Status()
			stopped := false
			for _, issue := range status.Issues {
				if issue.Filename == "" && issue.Reason == reasonStopped {
					stopped = true
				}
			}
			if client.searchCalls != tc.searches || status.EnrichmentFailed != tc.failed || status.EnrichmentUnmatched != tc.unmatched || stopped != tc.stopped || status.Imported != 5 || status.EnrichmentTotal != tc.total {
				t.Fatalf("searches=%d status=%+v stopped=%v", client.searchCalls, status, stopped)
			}
			if tc.stopped && status.EnrichmentProcessed >= status.EnrichmentTotal {
				t.Fatal("a stopped run must leave work unprocessed")
			}
		})
	}
}

// A failed season response fails the season and every pending episode that
// depended on it, once, with one issue naming the season.
func TestShowSeasonFailureCountsDependents(t *testing.T) {
	s, _, root := setupScanner(t)
	writeFile(t, root, "Example (2020)/Season 1/S01E01E02.mkv", "a")
	client := &testTMDB{}
	client.seasonHook = func(int, int) (*tmdb.TVSeason, error) { return nil, errors.New("season unavailable") }
	s.Tmdb = client
	scanOK(t, s, root)
	status := s.Status()
	if status.Enriched != 1 || status.EnrichmentFailed != 3 || status.EnrichmentProcessed != 4 || status.EnrichmentTotal != 4 || status.IssueCount != 1 || status.PendingEnrichment != 3 {
		t.Fatalf("%+v", status)
	}
}
