package movie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/tmdb"

	sqlite3 "github.com/mattn/go-sqlite3"
)

func createMovieLibrary(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	for i := range count {
		path := filepath.Join(root, fmt.Sprintf("group-%02d", i%10), "nested", fmt.Sprintf("movie-%03d.mkv", i))
		err := os.MkdirAll(filepath.Dir(path), 0700)
		if err != nil {
			t.Fatal(err)
		}
		scannertest.WriteFile(t, path, "media")
	}
	return root
}

// awaitActiveFiles waits for the coordinator to publish count active files and
// reports whether it got there. The probe stub signals from inside the worker,
// but the dispatch loop records the activation afterwards on its own goroutine,
// so a probe that has started is not yet a file the status reports. Polling
// closes that window; reading the status once races the coordinator.
//
// It returns rather than failing so the caller can still stop the scan it
// started before the test goroutine exits.
func awaitActiveFiles(s *Scanner, count int) (Status, bool) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		status := s.Status()
		if len(status.ActiveFiles) == count {
			return status, true
		}
		if time.Now().After(deadline) {
			return status, false
		}
		time.Sleep(time.Millisecond)
	}
}

func TestMoviePipeline418Files(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%v", canceled), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			root := createMovieLibrary(t, 418)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.scanContext = ctx
			var active, maximum, calls atomic.Int32
			entered := make(chan struct{}, 418)
			release := make(chan struct{})
			s.ffprobe = &scannertest.Probe{Callback: func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
				calls.Add(1)
				current := active.Add(1)
				defer active.Add(-1)
				for previous := maximum.Load(); current > previous; previous = maximum.Load() {
					if maximum.CompareAndSwap(previous, current) {
						break
					}
				}
				entered <- struct{}{}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-release:
				}
				if strings.HasSuffix(path, "movie-017.mkv") {
					return nil, errors.New("isolated bad media at " + path)
				}
				return movieScannerMetadataFixture("120"), nil
			}}
			done := make(chan struct{})
			go func() { s.scan(root); close(done) }()
			scannertest.WaitForSignal(t, entered, 5*time.Second, "scan boundary")
			scannertest.WaitForSignal(t, entered, 5*time.Second, "scan boundary")
			status, bothActive := awaitActiveFiles(s, 2)
			if !bothActive || status.Total != 418 || status.Phase != "local" {
				cancel()
				scannertest.WaitForSignal(t, done, 5*time.Second, "scan boundary")
				t.Fatalf("discovery/active accounting: %+v", status)
			}
			// Snapshots are independent of the coordinator's state.
			status.ActiveFiles[0] = "mutated by reader"
			if s.Status().ActiveFiles[0] == "mutated by reader" {
				cancel()
				scannertest.WaitForSignal(t, done, 5*time.Second, "scan boundary")
				t.Fatal("status shares mutable active files")
			}
			if canceled {
				cancel()
			} else {
				close(release)
			}
			scannertest.WaitForSignal(t, done, 5*time.Second, "scan boundary")
			status = s.Status()
			if maximum.Load() != 2 || active.Load() != 0 || len(status.ActiveFiles) != 0 {
				t.Fatalf("workers not bounded/joined: %+v max=%d active=%d", status, maximum.Load(), active.Load())
			}
			if canceled {
				if status.State != "canceled" || status.Imported != 0 || calls.Load() != 2 {
					t.Fatalf("cancellation: %+v calls=%d", status, calls.Load())
				}
				return
			}
			if status.State != "completed-with-issues" || status.Processed != 418 || status.Imported != 417 || status.Failed != 1 || status.Deferred != 0 || status.PendingEnrichment != 417 {
				t.Fatalf("incomplete accounting: %+v", status)
			}
			if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies") != 417 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM video_streams") != 417 {
				t.Fatal("committed catalog incomplete")
			}
			if len(status.Issues) != 1 || status.Issues[0].Filename != "movie-017.mkv" || strings.Contains(status.Issues[0].Reason, root) {
				t.Fatalf("unsafe issues: %+v", status.Issues)
			}
			finished := 0
			for _, entry := range s.logger.(*scannertest.Logger).InfoEntries {
				if entry.Msg == "movie scan finished" {
					finished++
				}
			}
			if finished != 1 {
				t.Fatalf("scan completion logged %d times, want once", finished)
			}
			s.scan(root)
			status = s.Status()
			if status.Unchanged != 417 || status.Failed != 1 || calls.Load() != 419 {
				t.Fatalf("unchanged rescan reprobed movies: %+v calls=%d", status, calls.Load())
			}
		})
	}
}

func TestLocalMoviesAvailableBeforeBlockedEnrichmentAndCancellation(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	root := createMovieLibrary(t, 5)
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	entered := make(chan struct{}, 2)
	var active atomic.Int32
	s.tmdb = &stubMovieScannerTmdb{searchHook: func(ctx context.Context, _ string, _ int) ([]tmdb.TmdbMovie, error) {
		active.Add(1)
		defer active.Add(-1)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > scanner.TmdbLookupTimeout {
			t.Error("resolution is not bounded by the TMDB lookup timeout")
		}
		entered <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	done := make(chan struct{})
	go func() { s.scan(root); close(done) }()
	scannertest.WaitForSignal(t, entered, 5*time.Second, "scan boundary")
	scannertest.WaitForSignal(t, entered, 5*time.Second, "scan boundary")
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies") != 5 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM video_streams") != 5 {
		t.Error("TMDB blocked local availability")
	}
	status := s.Status()
	if status.Imported != 5 || status.Phase != "enrichment" || status.PendingEnrichment != 5 {
		t.Errorf("local progress: %+v", status)
	}
	cancel()
	scannertest.WaitForSignal(t, done, 5*time.Second, "scan boundary")
	if s.Status().State != "canceled" || active.Load() != 0 || s.Status().EnrichmentFailed != 0 {
		t.Fatalf("request cancellation: %+v active=%d", s.Status(), active.Load())
	}
}

func TestEnrichmentOutagesNoMatchesAndRecovery(t *testing.T) {
	for _, providerErr := range []error{&tmdb.StatusError{StatusCode: http.StatusUnauthorized}, &tmdb.StatusError{StatusCode: http.StatusTooManyRequests}, &tmdb.StatusError{StatusCode: http.StatusServiceUnavailable}, context.DeadlineExceeded, tmdb.ErrNoMoviesFound} {
		t.Run(fmt.Sprintf("%T-%v", providerErr, providerErr), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			root := createMovieLibrary(t, 20)
			probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
			s.ffprobe = probe
			var calls atomic.Int32
			s.tmdb = &stubMovieScannerTmdb{searchHook: func(context.Context, string, int) ([]tmdb.TmdbMovie, error) { calls.Add(1); return nil, providerErr }}
			s.scan(root)
			status := s.Status()
			noMatch := errors.Is(providerErr, tmdb.ErrNoMoviesFound)
			authentication, _ := tmdb.ProviderFailure(providerErr)
			if noMatch {
				if calls.Load() != 20 || status.EnrichmentFailed != 0 || status.EnrichmentUnmatched != 20 {
					t.Fatalf("no-match treated as outage: %+v calls=%d", status, calls.Load())
				}
			} else {
				limit := int32(4)
				if authentication {
					limit = 2
				}
				if calls.Load() > limit || status.EnrichmentFailed == 0 {
					t.Fatalf("outage dispatch not stopped: %+v calls=%d", status, calls.Load())
				}
			}
			if status.Imported != 20 || status.PendingEnrichment != 20 {
				t.Fatalf("provider failure blocked local import: %+v", status)
			}
			details := retryMovieFixture(t)
			s.tmdb = &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Movie"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}
			s.scan(root)
			status = s.Status()
			if status.Enriched != 20 || status.Unchanged != 20 || status.State != "completed" || probe.Calls() != 20 || status.PendingEnrichment != 0 {
				t.Fatalf("metadata-only recovery: %+v probes=%d", status, probe.Calls())
			}
			s.scan(root)
			if s.Status().EnrichmentTotal != 0 || probe.Calls() != 20 {
				t.Fatal("unchanged identified movies triggered unnecessary work")
			}
		})
	}
}

func TestMovieStatusIssueLimitAndFatalDiscovery(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		return nil, errors.New("secret raw error")
	}}
	s.scan(createMovieLibrary(t, 110))
	status := s.Status()
	if status.IssueCount != 110 || len(status.Issues) != 100 || status.Failed != 110 {
		t.Fatalf("issue cap: %+v", status)
	}
	if strings.Contains(status.Issues[0].Reason, "secret raw error") {
		t.Fatalf("issue leaks the raw probe error: %+v", status.Issues[0])
	}
	s.scan(filepath.Join(t.TempDir(), "missing"))
	status = s.Status()
	if status.State != "failed" || status.Total != 0 || status.RunID == "" || status.FinishedAt == nil {
		t.Fatalf("fatal discovery: %+v", status)
	}
}

// A library scan and a startup catch-up, which no watcher takes deferred files
// from, retry their own.
func TestDeferredRetryEligibilityAndAccounting(t *testing.T) {
	for _, tc := range []struct {
		unstable, startup bool
	}{{false, false}, {true, false}, {false, true}, {true, true}} {
		unstable := tc.unstable
		t.Run(fmt.Sprintf("unstable=%v/startup=%v", tc.unstable, tc.startup), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			root := createMovieLibrary(t, 1)
			now := time.Now()
			s.now = func() time.Time { return now }
			waits := 0
			s.waitForRetry = func(ctx context.Context, delay time.Duration) error {
				if delay <= 0 || delay > 60*time.Second {
					t.Errorf("quiet-period wait=%v", delay)
				}
				waits++
				now = now.Add(delay)
				return ctx.Err()
			}
			calls := 0
			s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
				calls++
				if unstable {
					return nil, &scanner.FileDeferral{Reason: scanner.FileChanged}
				}
				return movieScannerMetadataFixture("120"), nil
			}}
			if tc.startup {
				s.scanChanges(root, scanner.Changes{CatchUp: true})
			} else {
				s.scan(root)
			}
			status := s.Status()
			if status.Total != 1 || status.Processed != 1 || status.Failed != 0 {
				t.Fatalf("retry counted a file twice: %+v", status)
			}
			if unstable {
				if waits != 2 || calls != 2 || status.Deferred != 1 || status.IssueCount != 1 {
					t.Fatalf("unbounded retries: waits=%d calls=%d status=%+v", waits, calls, status)
				}
			} else if waits != 1 || calls != 1 || status.Imported != 1 || status.Deferred != 0 || status.IssueCount != 0 {
				t.Fatalf("eligible retry failed: waits=%d calls=%d status=%+v", waits, calls, status)
			}
		})
	}
}

func TestFailedMovieProbeValidation(t *testing.T) {
	for _, test := range []struct {
		name       string
		mutation   string
		failures   int
		wantCalls  int
		wantWaits  int
		wantFailed int
		wantDefer  int
	}{
		{name: "truncate then recover", mutation: "truncate", failures: 1, wantCalls: 2, wantWaits: 1},
		{name: "replace then recover", mutation: "replace", failures: 1, wantCalls: 2, wantWaits: 1},
		{name: "truncate through retry limit", mutation: "truncate", failures: 3, wantCalls: 3, wantWaits: 2, wantDefer: 1},
		{name: "replace through retry limit", mutation: "replace", failures: 3, wantCalls: 3, wantWaits: 2, wantDefer: 1},
		{name: "stable probe failure", failures: 1, wantCalls: 1, wantFailed: 1},
	} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%v", test.name, existing), func(t *testing.T) {
				fixture := setupMovieScanner(t)
				s := fixture.scanner
				root := t.TempDir()
				path := filepath.Join(root, "movie.mkv")
				scannertest.WriteFile(t, path, "original media")
				assertPreserved := func() {
					t.Helper()
					if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies") != 0 {
						t.Error("failed probe imported a movie")
					}
				}
				if existing {
					s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
					s.scan(root)
					_, err := s.tx.DB.Exec("UPDATE movies SET title='retained', updated_at='2000-01-01 00:00:00'")
					if err != nil {
						t.Fatal(err)
					}
					before, err := readTestMovieByPath(context.Background(), s.queries, path)
					if err != nil {
						t.Fatal(err)
					}
					baseline, _, err := s.loadMovieScanIndex(context.Background(), nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					assertPreserved = func() {
						t.Helper()
						after, err := readTestMovieByPath(context.Background(), s.queries, path)
						if err != nil || !reflect.DeepEqual(before, after) {
							t.Errorf("failed probe changed catalog: before=%+v after=%+v err=%v", before, after, err)
						}
						stored, _, err := s.loadMovieScanIndex(context.Background(), nil, nil)
						if err != nil || !reflect.DeepEqual(baseline, stored) {
							t.Errorf("failed probe changed fingerprint: %v", err)
						}
					}
					// Force a technical update without changing the saved baseline.
					scannertest.WriteFile(t, path, "changed media")
				}
				// Start past initial eligibility so every deferral comes from a failed probe.
				now := scannertest.SettledNow()
				s.now = func() time.Time { return now }
				calls, waits, invalidations := 0, 0, 0
				s.invalidateCommittedMovie = func(int64) { invalidations++ }
				s.waitForRetry = func(ctx context.Context, delay time.Duration) error {
					status := s.Status()
					if delay != scanner.FileQuietPeriod || status.Deferred != 1 || status.Failed != 0 || status.Processed != 1 || status.IssueCount != 1 {
						t.Errorf("failed probe was not deferred until eligible: delay=%v status=%+v", delay, status)
					}
					assertPreserved()
					if invalidations != 0 {
						t.Error("failed probe invalidated playback caches")
					}
					waits++
					now = now.Add(delay)
					return ctx.Err()
				}
				s.ffprobe = &scannertest.Probe{Callback: func(_ context.Context, path string) (*ffprobe.FfprobeResult, error) {
					calls++
					if calls > test.failures {
						return movieScannerMetadataFixture("240"), nil
					}
					var err error
					switch test.mutation {
					case "truncate":
						err = os.Truncate(path, int64(4-calls))
					case "replace":
						err = os.WriteFile(path+".new", []byte("replacement media"), 0600)
						if err == nil {
							err = os.Rename(path+".new", path)
						}
					}
					if err != nil {
						t.Errorf("mutate file during probe: %v", err)
					}
					return nil, errors.New("ordinary probe failure")
				}}
				s.scan(root)
				status := s.Status()
				if calls != test.wantCalls || waits != test.wantWaits || status.Total != 1 || status.Processed != 1 || status.Failed != test.wantFailed || status.Deferred != test.wantDefer {
					t.Fatalf("probe retry accounting: calls=%d waits=%d status=%+v", calls, waits, status)
				}
				if test.wantFailed+test.wantDefer > 0 {
					assertPreserved()
					if invalidations != 0 || status.Imported != 0 || status.Updated != 0 || status.IssueCount != 1 || status.State != "completed-with-issues" {
						t.Fatalf("failed probe outcome: invalidations=%d status=%+v", invalidations, status)
					}
					return
				}
				wantImported, wantUpdated := 1, 0
				if existing {
					wantImported, wantUpdated = 0, 1
				}
				if status.Imported != wantImported || status.Updated != wantUpdated || status.IssueCount != 0 || len(status.Issues) != 0 || invalidations != 1 {
					t.Fatalf("recovery outcome: invalidations=%d status=%+v", invalidations, status)
				}
				if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies WHERE duration=240") != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movie_file_fingerprints") != 1 {
					t.Fatal("stable retry did not commit movie and fingerprint")
				}
			})
		}
	}
}

func TestCancellationDuringDeferredWait(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	s.now = time.Now
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	entered := make(chan struct{}, 1)
	s.waitForRetry = func(ctx context.Context, delay time.Duration) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		return waitForMovieRetry(ctx, delay)
	}
	done := make(chan struct{})
	go func() { s.scan(root); close(done) }()
	scannertest.WaitForSignal(t, entered, 5*time.Second, "scan boundary")
	if s.Status().Phase != "retry-wait" {
		t.Error("missing retry phase")
	}
	cancel()
	scannertest.WaitForSignal(t, done, 5*time.Second, "scan boundary")
	if s.Status().State != "canceled" || s.Status().Deferred != 1 {
		t.Fatalf("retry cancellation: %+v", s.Status())
	}
}

func TestCancellationImmediatelyAfterCommitRetainsAccounting(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	root := createMovieLibrary(t, 5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	s.invalidateCommittedMovie = func(int64) { cancel() }
	s.scan(root)
	status := s.Status()
	if status.State != "canceled" || status.Imported != 1 || status.Processed != 1 || status.PendingEnrichment != 1 {
		t.Fatalf("lost committed accounting: %+v", status)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies") != 1 {
		t.Fatal("committed movie was lost")
	}
}

func TestCancellationDuringTechnicalTransaction(t *testing.T) {
	fixture := setupMovieScannerDatabase(t, filepath.Join(t.TempDir(), "cancel.db")+"?_foreign_keys=on")
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	conn, err := s.tx.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Raw(func(raw any) error {
		return raw.(*sqlite3.SQLiteConn).RegisterFunc("cancel_scan", func() int { cancel(); return 0 }, false)
	})
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.tx.DB.Exec("CREATE TRIGGER cancel_technical AFTER INSERT ON movie_file_fingerprints BEGIN SELECT cancel_scan(); END")
	if err != nil {
		t.Fatal(err)
	}
	s.scan(root)
	if s.Status().State != "canceled" || s.Status().Imported != 0 {
		t.Fatalf("transaction cancellation: %+v", s.Status())
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies") != 0 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM video_streams") != 0 {
		t.Fatal("canceled transaction committed partial movie")
	}
}

func TestEnrichmentRechecksPersistedBaseline(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	s.scan(root)
	client := &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42}}, detailMovies: map[int]tmdb.TmdbMovie{42: retryMovieFixture(t)}}
	client.detailHook = func() {
		_, err := s.tx.DB.Exec("UPDATE movie_file_fingerprints SET inode='replacement'")
		if err != nil {
			t.Error(err)
		}
	}
	s.tmdb = client
	s.scan(root)
	if s.Status().Enriched != 0 || s.Status().PendingEnrichment != 1 {
		t.Fatalf("stale baseline accepted: %+v", s.Status())
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM movies WHERE tmdb_id IS NOT NULL") != 0 {
		t.Fatal("stale enrichment overwrote catalog")
	}
}

func TestRetryWindowStopsDispatchingDeferredFiles(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	root := createMovieLibrary(t, 10)
	var clock atomic.Int64
	clock.Store(time.Now().UnixNano())
	s.now = func() time.Time { return time.Unix(0, clock.Load()) }
	s.waitForRetry = func(ctx context.Context, delay time.Duration) error { clock.Add(int64(delay)); return ctx.Err() }
	var calls atomic.Int32
	s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		calls.Add(1)
		clock.Add(int64(121 * time.Second))
		return movieScannerMetadataFixture("120"), nil
	}}
	s.scan(root)
	status := s.Status()
	if calls.Load() > 2 || status.Imported < 1 || status.Processed != 10 || status.Imported+status.Deferred != 10 {
		t.Fatalf("dispatched work after retry window: calls=%d status=%+v", calls.Load(), status)
	}
}

func TestMovieReplacementAndSymlinkRetarget(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprint(symlink), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
			s.ffprobe = probe
			root := t.TempDir()
			path := filepath.Join(root, "movie.mkv")
			source := path
			if symlink {
				source = filepath.Join(root, "original.data")
			}
			scannertest.WriteFile(t, source, "identical movie")
			if symlink {
				err := os.Symlink(source, path)
				if err != nil {
					t.Fatal(err)
				}
			}
			s.scan(root)
			original, err := readTestMovieByPath(context.Background(), s.queries, path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			replacement := filepath.Join(root, "replacement.data")
			scannertest.WriteFile(t, replacement, "identical movie")
			err = os.Chtimes(replacement, info.ModTime(), info.ModTime())
			if err != nil {
				t.Fatal(err)
			}
			if symlink {
				err = os.Remove(path)
				if err == nil {
					err = os.Symlink(replacement, path)
				}
			} else {
				err = os.Rename(replacement, path)
			}
			if err != nil {
				t.Fatal(err)
			}
			invalidations := 0
			s.invalidateCommittedMovie = func(int64) { invalidations++ }
			s.scan(root)
			current, err := readTestMovieByPath(context.Background(), s.queries, path)
			if err != nil {
				t.Fatal(err)
			}
			if current.ID != original.ID || probe.Calls() != 2 || invalidations != 1 || s.Status().Updated != 1 {
				t.Fatalf("replacement was missed: %+v probes=%d invalidations=%d", s.Status(), probe.Calls(), invalidations)
			}
		})
	}
}

// Workers must not touch the database: the pool is pinned to one connection,
// so a worker read would wait behind the coordinator's open transaction before
// it could even start ffprobe. A closed database fails every query, so a
// successful prepare proves the worker stayed off it.
func TestWorkersPrepareWithoutDatabase(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("3600")}
	details := retryMovieFixture(t)
	s.tmdb = &stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42, Title: "Local"}}, detailMovies: map[int]tmdb.TmdbMovie{42: details}}
	path := filepath.Join(t.TempDir(), "Local (2001).mkv")
	scannertest.WriteFile(t, path, "movie")
	fixture.db.Close()
	_, err := s.queries.GetMovieByPath(context.Background(), path)
	if err == nil {
		t.Fatal("closed database still answers queries")
	}

	file := scanner.ScanFile{Path: path, Ext: "mkv"}
	probed := s.prepareFile(context.Background(), probeJob{file: file})
	if probed.err != nil || probed.resolved == nil || probed.resolved.params.Title != "Local" {
		t.Fatalf("probe used the database: err=%v resolved=%+v", probed.err, probed.resolved)
	}
	probed.inspection.Close()

	inspection, err := scanner.InspectFileMetadata(context.Background(), path, nil, s.now, scanner.FileQuietPeriod)
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

func catalogPaths(t *testing.T, s *Scanner) []string {
	t.Helper()
	rows, err := s.tx.DB.Query("SELECT file_path FROM movies ORDER BY file_path")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	err = rows.Err()
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestMovieChangesRunAppliesOnlyTheChangedPaths(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	s.ffprobe = probe
	root := t.TempDir()
	a := scannertest.WriteFile(t, filepath.Join(root, "A (2001)", "a.mkv"), "a")
	b := scannertest.WriteFile(t, filepath.Join(root, "B (2002)", "b.mkv"), "b")
	c := scannertest.WriteFile(t, filepath.Join(root, "C (2003)", "c.mkv"), "c")
	s.scan(root)
	if probe.Calls() != 3 {
		t.Fatalf("library scan probes = %d", probe.Calls())
	}

	scannertest.WriteFile(t, a, "a rewritten")
	d := scannertest.WriteFile(t, filepath.Join(root, "D (2004)", "d.mkv"), "d")
	err := os.RemoveAll(filepath.Dir(c))
	if err != nil {
		t.Fatal(err)
	}
	// Gone too, but the watcher did not report it: a run of changes leaves it.
	err = os.Remove(b)
	if err != nil {
		t.Fatal(err)
	}
	outside := scannertest.WriteFile(t, filepath.Join(filepath.Dir(root), filepath.Base(root)+"-outside", "e.mkv"), "e")

	s.scanChanges(root, scanner.Changes{Paths: []string{
		"A (2001)/a.mkv",
		"D (2004)",
		"C (2003)",
		filepath.Join("..", filepath.Base(root)+"-outside", filepath.Base(outside)),
	}})

	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.State != scanner.StateCompleted {
		t.Fatalf("status scope=%s state=%s issues=%+v", status.Scope, status.State, status.Issues)
	}
	if status.Total != 2 || status.Imported != 1 || status.Updated != 1 || status.Deleted != 1 {
		t.Fatalf("counts = %+v", status.Counts)
	}
	// The run read only its own rows, but pending enrichment still counts the
	// whole catalog: no TMDB client, so every remaining movie is pending.
	if status.PendingEnrichment != 3 {
		t.Fatalf("pending enrichment = %d, want 3", status.PendingEnrichment)
	}
	if probe.Calls() != 5 {
		t.Fatalf("probes = %d, want only the two changed files probed", probe.Calls())
	}
	want := []string{a, b, d}
	sort.Strings(want)
	got := catalogPaths(t, s)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog = %q, want %q", got, want)
	}
}

func TestMovieChangesRunUsesTheShortQuietPeriodAndHandsBackDeferrals(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "Fresh (2020)", "fresh.mkv"), "fresh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Ten seconds after the write the watcher's run imports the file, while
	// a library scan would still defer it.
	s.now = func() time.Time { return info.ModTime().Add(10 * time.Second) }
	s.scanChanges(root, scanner.Changes{Paths: []string{"Fresh (2020)/fresh.mkv"}})
	if status := s.Status(); status.Imported != 1 || status.Deferred != 0 {
		t.Fatalf("10s after the write: %+v", status.Counts)
	}

	// One second after a rewrite it is deferred and handed back, and the run
	// ends without waiting through the retry window.
	scannertest.WriteFile(t, path, "fresh, rewritten")
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return info.ModTime().Add(time.Second) }
	var mu sync.Mutex
	var deferred []string
	var eligibleAt []time.Time
	changes := scanner.Changes{
		Paths: []string{"Fresh (2020)/fresh.mkv"},
		OnDeferred: func(path string, eligible time.Time) {
			mu.Lock()
			defer mu.Unlock()
			deferred = append(deferred, path)
			eligibleAt = append(eligibleAt, eligible)
		},
	}
	started := time.Now()
	s.scanChanges(root, changes)
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("the run waited %s; a watcher run must not hold the launcher through the retry window", elapsed)
	}
	status := s.Status()
	if status.Deferred != 1 || status.Updated != 0 {
		t.Fatalf("1s after the rewrite: %+v", status.Counts)
	}
	if !reflect.DeepEqual(deferred, []string{"Fresh (2020)/fresh.mkv"}) {
		t.Fatalf("deferred = %q", deferred)
	}
	// Eligible ChangeQuietPeriod after the later of mtime and ctime.
	earliest := info.ModTime().Add(scanner.ChangeQuietPeriod)
	latest := time.Now().Add(scanner.ChangeQuietPeriod)
	if eligibleAt[0].Before(earliest) || eligibleAt[0].After(latest) {
		t.Fatalf("eligible at %v, want within [%v, %v]", eligibleAt[0], earliest, latest)
	}
	if status.Issues[0].Reason != scanner.ReasonFileDeferredWatched {
		t.Fatalf("issue = %+v", status.Issues)
	}
}

func TestMovieCatchUpRefusesToEmptyALibraryThatLooksUnmounted(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "A (2001)", "a.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(root, "B (2002)", "b.mkv"), "b")
	s.scan(root)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		err = os.RemoveAll(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
	}

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if len(catalogPaths(t, s)) != 2 || status.Deleted != 0 || status.Scope != scanner.ScopeLibrary {
		t.Fatalf("catch-up removed rows: deleted=%d scope=%s", status.Deleted, status.Scope)
	}
	if status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupSkippedEmpty {
		t.Fatalf("issues = %+v", status.Issues)
	}

	// A library scan an admin asked for still trusts the empty walk.
	s.scan(root)
	if len(catalogPaths(t, s)) != 0 {
		t.Fatal("a requested library scan must still remove missing movies")
	}
}

// A disk mounted inside the library that has not come up, after a power cut
// on a server whose disks are mounted one per folder, leaves its empty mount
// point. Every catch-up keeps its movies until it is back, while a movie
// deleted from a disk that is mounted still goes; a library scan an admin
// asks for deletes them.
func TestMovieCatchUpKeepsTheMoviesOfADiskThatIsNotMounted(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	disk1, disk2 := filepath.Join(root, "disk1"), filepath.Join(root, "disk2")
	kept := filepath.Join(disk1, "A (2001)", "a.mkv")
	deleted := filepath.Join(disk1, "D (2004)", "d.mkv")
	unmounted := []string{filepath.Join(disk2, "B (2002)", "b.mkv"), filepath.Join(disk2, "C (2003)", "c.mkv")}
	for _, path := range append([]string{kept, deleted}, unmounted...) {
		scannertest.WriteFile(t, path, filepath.Base(path))
	}
	// A test cannot mount a filesystem, so the mount table is scripted.
	mounted := func(points ...string) func() ([]string, error) {
		return func() ([]string, error) { return points, nil }
	}
	s.mountPoints = mounted(disk1, disk2)
	s.scan(root)
	for _, directory := range []string{disk2, filepath.Dir(deleted)} {
		err = os.RemoveAll(directory)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = os.Mkdir(disk2, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	s.mountPoints = mounted(disk1)

	for run := range 2 {
		s.scanChanges(root, scanner.Changes{CatchUp: true})
		status := s.Status()
		got, want := catalogPaths(t, s), append([]string{kept}, unmounted...)
		if !slices.Equal(got, want) {
			t.Fatalf("after catch-up %d: %q, want %q", run+1, got, want)
		}
		if status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupKeptUnmounted || status.Issues[0].Filename != "disk2" {
			t.Fatalf("catch-up %d issues = %+v", run+1, status.Issues)
		}
	}

	s.scan(root)
	got := catalogPaths(t, s)
	if !slices.Equal(got, []string{kept}) {
		t.Fatalf("after the library scan: %q", got)
	}
}

func TestMovieReprobeKeepsTheTmdbMissBackoff(t *testing.T) {
	for _, changesOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "library", true: "changes"}[changesOnly], func(t *testing.T) {
			fixture := setupMovieScanner(t)
			s := fixture.scanner
			probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
			s.ffprobe = probe
			client := &stubMovieScannerTmdb{searchErr: tmdb.ErrNoMoviesFound}
			s.tmdb = client
			now := scannertest.SettledNow()
			s.now = func() time.Time { return now }
			root := t.TempDir()
			path := scannertest.WriteFile(t, filepath.Join(root, "Unknown (1999).mkv"), "unknown")
			scan := func() {
				if changesOnly {
					s.scanChanges(root, scanner.Changes{Paths: []string{"Unknown (1999).mkv"}})
				} else {
					s.scan(root)
				}
			}
			// Two actual misses establish the backoff, with a configured client.
			scan()
			scan()
			searches := len(client.searchCalls)
			if searches != 2 {
				t.Fatalf("searches before rewrite = %d", searches)
			}
			scannertest.WriteFile(t, path, "unknown, remuxed")
			scan()
			status := s.Status()
			if status.Updated != 1 || status.EnrichmentTotal != 0 || status.PendingEnrichment != 1 || status.State != scanner.StateCompleted {
				t.Fatalf("rewrite during backoff: %+v", status)
			}
			if probe.Calls() != 2 || len(client.searchCalls) != searches {
				t.Fatalf("rewrite: probes=%d searches=%d", probe.Calls(), len(client.searchCalls))
			}
			var attempts, lastAttempt int64
			err := s.tx.DB.QueryRow("SELECT attempts, last_attempt_at FROM movie_tmdb_retries").Scan(&attempts, &lastAttempt)
			if err != nil {
				t.Fatal(err)
			}
			if attempts != 2 || lastAttempt != now.Unix() {
				t.Fatalf("backoff changed: attempts=%d last=%d", attempts, lastAttempt)
			}
			now = now.Add(25 * time.Hour)
			scan()
			if len(client.searchCalls) != searches+1 || s.Status().EnrichmentTotal != 1 || probe.Calls() != 2 {
				t.Fatalf("expired backoff: %+v searches=%d probes=%d", s.Status(), len(client.searchCalls), probe.Calls())
			}
		})
	}
}

func TestMovieAtomicReplacementRefreshesOnlyChangedTechnicalState(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	probe := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	s.ffprobe = probe
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "movie.mkv"), "first")
	neighbor := scannertest.WriteFile(t, filepath.Join(root, "neighbor.mkv"), "neighbor")
	s.scan(root)
	before := nextMovieScan(t, s).movieIndex
	id := before[path].ID
	seedMoviePlaybackWork(t, s, id)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	temp := scannertest.WriteFile(t, filepath.Join(root, "download.part"), "other")
	err = os.Chtimes(temp, info.ModTime(), info.ModTime())
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(temp, path)
	if err != nil {
		t.Fatal(err)
	}
	probe.Default = movieScannerMetadataFixture("240")
	var invalidated []int64
	s.invalidateCommittedMovie = func(movieID int64) {
		invalidated = append(invalidated, movieID)
		movie, err := s.queries.GetMovieByID(context.Background(), movieID)
		if err != nil || movie.Duration.Float64 != 240 {
			t.Errorf("cache invalidation preceded technical commit: duration=%v err=%v", movie.Duration, err)
		}
	}
	// Polling reconsiders every media name in the changed directory.
	s.scanChanges(root, scanner.Changes{Paths: []string{"movie.mkv", "neighbor.mkv"}})
	status := s.Status()
	if status.Updated != 1 || status.Unchanged != 1 || probe.Calls() != 3 {
		t.Fatalf("replacement: %+v probes=%d", status, probe.Calls())
	}
	after := nextMovieScan(t, s).movieIndex
	if after[path].ID != id || after[path].FileFingerprint == before[path].FileFingerprint || after[neighbor] != before[neighbor] {
		t.Fatalf("replacement fingerprints: before=%+v after=%+v", before, after)
	}
	if !reflect.DeepEqual(invalidated, []int64{id}) {
		t.Fatalf("invalidated movies: %v", invalidated)
	}
	if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM remux_safety_verdicts") != 0 {
		t.Fatal("replacement retained remux_safety_verdicts")
	}
}

// A run of changes publishes its own report, but a library scan's failed file
// it did not look at is still failing, so its issue stays until a run looks at
// the file again.
func TestMovieChangesRunKeepsTheIssuesOfFilesItDidNotLookAt(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	broken := true
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120"), Hook: func(_ context.Context, path string) (*ffprobe.FfprobeResult, error) {
		if broken && filepath.Base(path) == "bad.mkv" {
			return nil, errors.New("unreadable")
		}
		return movieScannerMetadataFixture("120"), nil
	}}
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "Bad (2001)", "bad.mkv"), "bad")
	s.scan(root)
	if s.Status().IssueCount != 1 {
		t.Fatalf("library scan issues = %+v", s.Status().Issues)
	}
	scannertest.WriteFile(t, filepath.Join(root, "New (2002)", "new.mkv"), "new")
	s.scanChanges(root, scanner.Changes{Paths: []string{"New (2002)/new.mkv"}})
	status := s.Status()
	if status.IssueCount != 1 || status.Issues[0].Filename != "bad.mkv" {
		t.Fatalf("after a run of changes elsewhere: %+v", status.Issues)
	}
	// The run itself went fine.
	if status.State != scanner.StateCompleted {
		t.Fatalf("a run of changes with only carried issues ended %s", status.State)
	}
	broken = false
	s.scanChanges(root, scanner.Changes{Paths: []string{"Bad (2001)/bad.mkv"}})
	if s.Status().IssueCount != 0 {
		t.Fatalf("after the file was looked at again: %+v", s.Status().Issues)
	}
}

// A run of changes that cannot read a changed directory keeps listing the
// failures an earlier scan found inside it, beside its own issue for the
// directory.
func TestMovieChangesRunKeepsTheIssuesInsideADirectoryItCannotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory whatever its mode")
	}
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Hook: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		return nil, errors.New("unreadable")
	}}
	root := t.TempDir()
	directory := filepath.Join(root, "Bad (2001)")
	scannertest.WriteFile(t, filepath.Join(directory, "bad.mkv"), "bad")
	s.scan(root)
	err := os.Chmod(directory, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })
	s.scanChanges(root, scanner.Changes{Paths: []string{"Bad (2001)"}})
	var names []string
	for _, issue := range s.Status().Issues {
		names = append(names, issue.Filename)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Bad (2001)", "bad.mkv"}) {
		t.Fatalf("issues after the unreadable run: %q", names)
	}
}
