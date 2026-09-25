package show

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/tmdb"
)

// The enrichment total is known before the first request, so the progress
// bar can move; every entity then reports an outcome.
func TestShowEnrichmentTotalPublishedBeforeWork(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E03.mkv"), "b")
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
	scannertest.WriteFile(t, filepath.Join(root, "Unknown (2001)/Season 1/S01E01.mkv"), "a")
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
				scannertest.WriteFile(t, filepath.Join(root, show+"/Season 1/S01E01.mkv"), show)
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

// A definitive no-match between transient failures is a healthy provider
// answer: it resets the consecutive count, so alternating outcomes never trip
// the breaker.
func TestShowEnrichmentBreakerResetsOnNoMatch(t *testing.T) {
	s, _, root := setupScanner(t)
	for _, show := range []string{"A", "B", "C", "D", "E"} {
		scannertest.WriteFile(t, filepath.Join(root, show+"/Season 1/S01E01.mkv"), show)
	}
	client := &testTMDB{}
	client.searchHook = func(string, int) ([]tmdb.TVShow, error) {
		if client.searchCalls%2 == 1 {
			return nil, &tmdb.StatusError{StatusCode: http.StatusServiceUnavailable}
		}
		return nil, tmdb.ErrNoShowsFound
	}
	s.Tmdb = client
	scanOK(t, s, root)
	status := s.Status()
	if client.searchCalls != 5 || status.EnrichmentFailed != 3 || status.EnrichmentUnmatched != 2 || status.EnrichmentProcessed != 5 {
		t.Fatalf("alternating outcomes tripped the breaker: searches=%d %+v", client.searchCalls, status)
	}
}

// A failed season response fails the season and every pending episode that
// depended on it, once, with one issue naming the season.
func TestShowSeasonFailureCountsDependents(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "a")
	client := &testTMDB{}
	client.seasonHook = func(int, int) (*tmdb.TVSeason, error) { return nil, errors.New("season unavailable") }
	s.Tmdb = client
	scanOK(t, s, root)
	status := s.Status()
	if status.Enriched != 1 || status.EnrichmentFailed != 3 || status.EnrichmentProcessed != 4 || status.EnrichmentTotal != 4 || status.IssueCount != 1 || status.PendingEnrichment != 3 {
		t.Fatalf("%+v", status)
	}
}

// A season payload that cannot be trusted fails the season and every pending
// episode that depended on it, and leaves the season unidentified.
func TestShowSeasonResponseValidation(t *testing.T) {
	episodes := func(season int, firstID int) []tmdb.TVEpisode {
		return []tmdb.TVEpisode{{ID: firstID, SeasonNumber: season, EpisodeNumber: 1, Name: "First"}, {ID: 1002, SeasonNumber: season, EpisodeNumber: 2, Name: "Second"}}
	}
	for _, tc := range []struct {
		name   string
		remote func(season int) *tmdb.TVSeason
	}{
		{"missing season identity", func(season int) *tmdb.TVSeason {
			return &tmdb.TVSeason{SeasonNumber: season, Episodes: episodes(season, 1001)}
		}},
		{"season number mismatch", func(season int) *tmdb.TVSeason {
			return &tmdb.TVSeason{ID: 101, SeasonNumber: season + 1, Episodes: episodes(season+1, 1001)}
		}},
		{"episode without identity", func(season int) *tmdb.TVSeason {
			return &tmdb.TVSeason{ID: 101, SeasonNumber: season, Episodes: episodes(season, 0)}
		}},
		{"episode from another season", func(season int) *tmdb.TVSeason {
			return &tmdb.TVSeason{ID: 101, SeasonNumber: season, Episodes: episodes(season+1, 1001)}
		}},
		{"duplicate episode number", func(season int) *tmdb.TVSeason {
			duplicate := episodes(season, 1001)
			duplicate[1].EpisodeNumber = 1
			return &tmdb.TVSeason{ID: 101, SeasonNumber: season, Episodes: duplicate}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "a")
			client := &testTMDB{}
			client.seasonHook = func(_, season int) (*tmdb.TVSeason, error) { return tc.remote(season), nil }
			s.Tmdb = client
			scanOK(t, s, root)
			status := s.Status()
			if status.Enriched != 1 || status.EnrichmentFailed != 3 || status.EnrichmentProcessed != 4 || status.IssueCount != 1 || status.PendingEnrichment != 3 {
				t.Fatalf("%+v", status)
			}
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_seasons WHERE tmdb_id IS NOT NULL") != 0 {
				t.Fatal("rejected season payload was applied")
			}
		})
	}
}

// Once a season is identified, a later payload naming a different season id
// must not be applied to it or to its newly pending episodes.
func TestShowChangedSeasonIdentityFailsPendingEpisodes(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01.mkv"), "a")
	client := &testTMDB{}
	s.Tmdb = client
	scanOK(t, s, root)
	if s.Status().Enriched != 3 {
		t.Fatalf("initial enrichment: %+v", s.Status())
	}
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E02.mkv"), "b")
	client.seasonHook = func(_, season int) (*tmdb.TVSeason, error) {
		remote, _ := (&testTMDB{}).GetSeasonDetails(context.Background(), 0, season)
		remote.ID = 999
		return remote, nil
	}
	scanOK(t, s, root)
	status := s.Status()
	if status.EnrichmentTotal != 1 || status.EnrichmentFailed != 1 || status.Enriched != 0 {
		t.Fatalf("changed season identity: %+v", status)
	}
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_seasons WHERE tmdb_id = 101") != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episodes WHERE episode_number = 2 AND tmdb_id IS NOT NULL") != 0 {
		t.Fatal("changed season payload was applied")
	}
}

// An episode whose stored identity no longer matches the season payload stays
// pending instead of being overwritten.
func TestShowChangedEpisodeIdentityStaysPending(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01.mkv"), "a")
	client := &testTMDB{}
	s.Tmdb = client
	scanOK(t, s, root)
	_, err := s.DB.Exec("INSERT INTO show_episode_tmdb_retries(episode_id) SELECT id FROM show_episodes WHERE episode_number = 1")
	if err != nil {
		t.Fatal(err)
	}
	client.seasonHook = func(_, season int) (*tmdb.TVSeason, error) {
		remote, _ := (&testTMDB{}).GetSeasonDetails(context.Background(), 0, season)
		remote.Episodes[0].ID = 5001
		return remote, nil
	}
	scanOK(t, s, root)
	status := s.Status()
	log := s.Logger.(*scannertest.Logger)
	if status.EnrichmentFailed != 1 || !log.WarnMentions("show episode enrichment pending", "TMDB episode identity changed") {
		t.Fatalf("changed episode identity: %+v warnings=%+v", status, log.WarnEntries)
	}
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episodes WHERE episode_number = 1 AND tmdb_id = 1001") != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_tmdb_retries") != 1 {
		t.Fatal("changed episode payload was applied or the retry was cleared")
	}
}

// commitMetadata re-reads the season and episode it is about to describe: a
// row that changed while the network request was in flight is left alone.
func TestCommitMetadataRejectsStaleSeasonAndEpisode(t *testing.T) {
	for _, tc := range []struct {
		name, mutation, untouched string
		failed, enriched, total   int
	}{
		{"season", "UPDATE show_seasons SET tmdb_id = 555", "SELECT count(*) FROM show_seasons WHERE tmdb_id = 555 AND name <> 'Enriched season'", 1, 1, 2},
		{"episode", "UPDATE show_episodes SET episode_number = 9 WHERE episode_number = 2", "SELECT count(*) FROM show_episodes WHERE episode_number = 9 AND tmdb_id IS NULL", 1, 3, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "a")
			client := &testTMDB{}
			client.seasonHook = func(_, season int) (*tmdb.TVSeason, error) {
				_, err := s.DB.Exec(tc.mutation)
				if err != nil {
					return nil, err
				}
				return (&testTMDB{}).GetSeasonDetails(context.Background(), 0, season)
			}
			s.Tmdb = client
			scanOK(t, s, root)
			status := s.Status()
			if status.EnrichmentFailed != tc.failed || status.Enriched != tc.enriched || status.EnrichmentTotal != tc.total {
				t.Fatalf("stale %s: %+v", tc.name, status)
			}
			if scannertest.CountRows(t, s.DB, tc.untouched) != 1 {
				t.Fatalf("stale %s row was overwritten", tc.name)
			}
		})
	}
}

// A details payload that does not describe the id that was asked for, or has
// no name, is not an identity the catalog can trust.
func TestLookupShowRejectsInvalidIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		show *tmdb.TVShow
	}{
		{"different id", &tmdb.TVShow{ID: 999, Name: "Other"}},
		{"blank name", &tmdb.TVShow{ID: 10, Name: "   "}},
		{"no payload", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "a")
			client := &testTMDB{}
			client.showHook = func(int) (*tmdb.TVShow, error) { return tc.show, nil }
			s.Tmdb = client
			scanOK(t, s, root)
			status := s.Status()
			if status.EnrichmentFailed != 1 || status.EnrichmentTotal != 1 || status.Enriched != 0 || client.seasonCalls != 0 {
				t.Fatalf("invalid identity: %+v seasonCalls=%d", status, client.seasonCalls)
			}
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows WHERE tmdb_id IS NOT NULL") != 0 {
				t.Fatal("invalid identity was applied")
			}
		})
	}
}

// A title with two interpretations searches both; a provider error on one is
// only reported when the other finds nothing either.
func TestLookupShowFallsBackAcrossInterpretations(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		second                      []tmdb.TVShow
		secondErr                   error
		failed, enriched, showCalls int
	}{
		{"error then no match", nil, tmdb.ErrNoShowsFound, 1, 0, 0},
		{"error then match", []tmdb.TVShow{{ID: 10, Name: "Space: 1999", FirstAirDate: "1975-09-04"}}, nil, 0, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Space 1999/Season 1/S01E01.mkv"), "a")
			client := &testTMDB{}
			client.searchHook = func(string, int) ([]tmdb.TVShow, error) {
				if client.searchCalls == 1 {
					return nil, &tmdb.StatusError{StatusCode: http.StatusServiceUnavailable}
				}
				return tc.second, tc.secondErr
			}
			s.Tmdb = client
			scanOK(t, s, root)
			status := s.Status()
			log := s.Logger.(*scannertest.Logger)
			if client.searchCalls != 2 || client.showCalls != tc.showCalls || status.EnrichmentFailed != tc.failed || status.Enriched != tc.enriched {
				t.Fatalf("searches=%d shows=%d status=%+v", client.searchCalls, client.showCalls, status)
			}
			if !log.WarnMentions("TMDB show search failed", "Space 1999") {
				t.Fatalf("search failure not logged: %+v", log.WarnEntries)
			}
		})
	}
}

// A miss that cannot be recorded is an enrichment failure, not an unmatched
// show: without the retry bookkeeping the backoff would never apply.
func TestShowUnmatchedSaveFailureCountsAsFailure(t *testing.T) {
	s, _, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Unknown (2001)/Season 1/S01E01.mkv"), "a")
	client := &testTMDB{}
	client.searchHook = func(string, int) ([]tmdb.TVShow, error) { return nil, tmdb.ErrNoShowsFound }
	s.Tmdb = client
	for _, trigger := range []string{
		"CREATE TRIGGER fail_miss_insert BEFORE INSERT ON show_tmdb_retries WHEN NEW.attempts > 0 BEGIN SELECT RAISE(ABORT,'forced'); END",
		"CREATE TRIGGER fail_miss_update BEFORE UPDATE ON show_tmdb_retries BEGIN SELECT RAISE(ABORT,'forced'); END",
	} {
		_, err := s.DB.Exec(trigger)
		if err != nil {
			t.Fatal(err)
		}
	}
	scanOK(t, s, root)
	status := s.Status()
	if status.EnrichmentFailed != 1 || status.EnrichmentUnmatched != 0 || status.EnrichmentTotal != 1 || status.IssueCount != 1 {
		t.Fatalf("unsaved miss: %+v", status)
	}
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_tmdb_retries WHERE attempts > 0") != 0 {
		t.Fatal("miss was recorded despite the failure")
	}
}
