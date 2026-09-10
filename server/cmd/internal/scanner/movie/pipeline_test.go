package movie

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"
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
		err = os.WriteFile(path, []byte("media"), 0600)
		if err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func awaitScanSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not reach expected boundary")
	}
}

func TestMoviePipeline418Files(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprintf("canceled=%v", canceled), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
			s := fixture.scanner
			root := createMovieLibrary(t, 418)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.scanContext = ctx
			var active, maximum, calls atomic.Int32
			entered := make(chan struct{}, 418)
			release := make(chan struct{})
			s.ffprobe = &fingerprintProbe{callback: func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
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
			go func() { s.runMovieScan(root); close(done) }()
			awaitScanSignal(t, entered)
			awaitScanSignal(t, entered)
			status := s.Status()
			if status.Total != 418 || status.Phase != "local" || len(status.ActiveFiles) != 2 {
				cancel()
				awaitScanSignal(t, done)
				t.Fatalf("discovery/active accounting: %+v", status)
			}
			// Snapshots are independent of the coordinator's state.
			status.ActiveFiles[0] = "mutated by reader"
			if canceled {
				cancel()
			} else {
				close(release)
			}
			awaitScanSignal(t, done)
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
			if countScannerRows(t, s.db, "SELECT count(*) FROM movies") != 417 || countScannerRows(t, s.db, "SELECT count(*) FROM video_streams") != 417 {
				t.Fatal("committed catalog incomplete")
			}
			if len(status.Issues) != 1 || status.Issues[0].Filename != "movie-017.mkv" || strings.Contains(status.Issues[0].Reason, root) {
				t.Fatalf("unsafe issues: %+v", status.Issues)
			}
			s.runMovieScan(root)
			status = s.Status()
			if status.Unchanged != 417 || status.Failed != 1 || calls.Load() != 419 {
				t.Fatalf("unchanged rescan reprobed movies: %+v calls=%d", status, calls.Load())
			}
		})
	}
}

type controlledTmdb struct {
	stubMovieScannerTmdb
	search func(context.Context) ([]tmdb.TmdbMovie, error)
}

func (c *controlledTmdb) SearchMoviesByTitleAndYear(ctx context.Context, _ string, _ ...int) ([]tmdb.TmdbMovie, error) {
	return c.search(ctx)
}

func TestLocalMoviesAvailableBeforeBlockedEnrichmentAndCancellation(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 5)
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	entered := make(chan struct{}, 2)
	var active atomic.Int32
	s.tmdb = &controlledTmdb{search: func(ctx context.Context) ([]tmdb.TmdbMovie, error) {
		active.Add(1)
		defer active.Add(-1)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Error("resolution has no 30-second budget")
		}
		entered <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	done := make(chan struct{})
	go func() { s.runMovieScan(root); close(done) }()
	awaitScanSignal(t, entered)
	awaitScanSignal(t, entered)
	if countScannerRows(t, s.db, "SELECT count(*) FROM movies") != 5 || countScannerRows(t, s.db, "SELECT count(*) FROM video_streams") != 5 {
		t.Error("TMDB blocked local availability")
	}
	status := s.Status()
	if status.Imported != 5 || status.Phase != "enrichment" || status.PendingEnrichment != 5 {
		t.Errorf("local progress: %+v", status)
	}
	cancel()
	awaitScanSignal(t, done)
	if s.Status().State != "canceled" || active.Load() != 0 || s.Status().EnrichmentFailed != 0 {
		t.Fatalf("request cancellation: %+v active=%d", s.Status(), active.Load())
	}
}

func TestEnrichmentOutagesNoMatchesAndRecovery(t *testing.T) {
	for _, providerErr := range []error{&tmdb.StatusError{StatusCode: http.StatusUnauthorized}, &tmdb.StatusError{StatusCode: http.StatusTooManyRequests}, &tmdb.StatusError{StatusCode: http.StatusServiceUnavailable}, context.DeadlineExceeded, tmdb.ErrNoMoviesFound} {
		t.Run(fmt.Sprintf("%T-%v", providerErr, providerErr), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
			s := fixture.scanner
			root := createMovieLibrary(t, 20)
			probe := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			s.ffprobe = probe
			var calls atomic.Int32
			s.tmdb = &controlledTmdb{search: func(context.Context) ([]tmdb.TmdbMovie, error) { calls.Add(1); return nil, providerErr }}
			s.runMovieScan(root)
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
			s.runMovieScan(root)
			status = s.Status()
			if status.Enriched != 20 || status.Unchanged != 20 || status.State != "completed" || probe.calls != 20 || status.PendingEnrichment != 0 {
				t.Fatalf("metadata-only recovery: %+v probes=%d", status, probe.calls)
			}
			s.runMovieScan(root)
			if s.Status().EnrichmentTotal != 0 || probe.calls != 20 {
				t.Fatal("unchanged identified movies triggered unnecessary work")
			}
		})
	}
}

func TestMovieStatusIssueLimitAndFatalDiscovery(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		return nil, errors.New("secret raw error")
	}}
	s.runMovieScan(createMovieLibrary(t, 110))
	status := s.Status()
	if status.IssueCount != 110 || len(status.Issues) != 100 || status.Failed != 110 {
		t.Fatalf("issue cap: %+v", status)
	}
	status.Issues[0].Reason = "mutated"
	if s.Status().Issues[0].Reason == "mutated" {
		t.Fatal("status shares mutable issues")
	}
	s.runMovieScan(filepath.Join(t.TempDir(), "missing"))
	status = s.Status()
	if status.State != "failed" || status.Total != 0 || status.RunID == "" || status.FinishedAt == nil {
		t.Fatalf("fatal discovery: %+v", status)
	}
}

func TestDeferredRetryEligibilityAndAccounting(t *testing.T) {
	for _, unstable := range []bool{false, true} {
		t.Run(fmt.Sprint(unstable), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
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
			s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
				calls++
				if unstable {
					return nil, &scanner.FileDeferral{Reason: scanner.FileChanged}
				}
				return movieScannerMetadataFixture("120"), nil
			}}
			s.runMovieScan(root)
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

func TestCancellationDuringDeferredWait(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	s.now = time.Now
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	entered := make(chan struct{})
	s.waitForRetry = func(ctx context.Context, delay time.Duration) error {
		close(entered)
		return waitForMovieRetry(ctx, delay)
	}
	done := make(chan struct{})
	go func() { s.runMovieScan(root); close(done) }()
	awaitScanSignal(t, entered)
	if s.Status().Phase != "retry-wait" {
		t.Error("missing retry phase")
	}
	cancel()
	awaitScanSignal(t, done)
	if s.Status().State != "canceled" || s.Status().Deferred != 1 {
		t.Fatalf("retry cancellation: %+v", s.Status())
	}
}

func TestCancellationImmediatelyAfterCommitRetainsAccounting(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 5)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	s.invalidateCommittedMovie = func(int64) { cancel() }
	s.runMovieScan(root)
	status := s.Status()
	if status.State != "canceled" || status.Imported != 1 || status.Processed != 1 || status.PendingEnrichment != 1 {
		t.Fatalf("lost committed accounting: %+v", status)
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM movies") != 1 {
		t.Fatal("committed movie was lost")
	}
}

func TestCancellationDuringTechnicalTransaction(t *testing.T) {
	fixture := setupMovieScannerDatabase(t, filepath.Join(t.TempDir(), "cancel.db")+"?_foreign_keys=on")
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	conn, err := s.db.Conn(ctx)
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
	_, err = s.db.Exec("CREATE TRIGGER cancel_technical AFTER INSERT ON movie_file_fingerprints BEGIN SELECT cancel_scan(); END")
	if err != nil {
		t.Fatal(err)
	}
	s.runMovieScan(root)
	if s.Status().State != "canceled" || s.Status().Imported != 0 {
		t.Fatalf("transaction cancellation: %+v", s.Status())
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM movies") != 0 || countScannerRows(t, s.db, "SELECT count(*) FROM video_streams") != 0 {
		t.Fatal("canceled transaction committed partial movie")
	}
}

func TestEnrichmentRechecksPersistedBaseline(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 1)
	s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	s.runMovieScan(root)
	client := &hookedMovieTmdb{stubMovieScannerTmdb: stubMovieScannerTmdb{searchResults: []tmdb.TmdbMovie{{TmdbID: 42}}, detailMovies: map[int]tmdb.TmdbMovie{42: retryMovieFixture(t)}}}
	client.hook = func() {
		_, err := s.db.Exec("UPDATE movie_file_fingerprints SET inode='replacement'")
		if err != nil {
			t.Error(err)
		}
	}
	s.tmdb = client
	s.runMovieScan(root)
	if s.Status().Enriched != 0 || s.Status().PendingEnrichment != 1 {
		t.Fatalf("stale baseline accepted: %+v", s.Status())
	}
	if countScannerRows(t, s.db, "SELECT count(*) FROM movies WHERE tmdb_id IS NOT NULL") != 0 {
		t.Fatal("stale enrichment overwrote catalog")
	}
}

func TestRetryWindowStopsDispatchingDeferredFiles(t *testing.T) {
	fixture := setupMovieScanner(t)
	defer fixture.db.Close()
	s := fixture.scanner
	root := createMovieLibrary(t, 10)
	var clock atomic.Int64
	clock.Store(time.Now().UnixNano())
	s.now = func() time.Time { return time.Unix(0, clock.Load()) }
	s.waitForRetry = func(ctx context.Context, delay time.Duration) error { clock.Add(int64(delay)); return ctx.Err() }
	var calls atomic.Int32
	s.ffprobe = &fingerprintProbe{callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		calls.Add(1)
		clock.Add(int64(121 * time.Second))
		return movieScannerMetadataFixture("120"), nil
	}}
	s.runMovieScan(root)
	status := s.Status()
	if calls.Load() > 2 || status.Imported < 1 || status.Processed != 10 || status.Imported+status.Deferred != 10 {
		t.Fatalf("dispatched work after retry window: calls=%d status=%+v", calls.Load(), status)
	}
}

func TestMovieReplacementAndSymlinkRetarget(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprint(symlink), func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
			s := fixture.scanner
			probe := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			s.ffprobe = probe
			root := t.TempDir()
			path := filepath.Join(root, "movie.mkv")
			source := path
			if symlink {
				source = filepath.Join(root, "original.data")
			}
			err := os.WriteFile(source, []byte("identical movie"), 0600)
			if err != nil {
				t.Fatal(err)
			}
			if symlink {
				err = os.Symlink(source, path)
				if err != nil {
					t.Fatal(err)
				}
			}
			s.runMovieScan(root)
			original, err := readTestMovieByPath(context.Background(), s.queries, path)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			replacement := filepath.Join(root, "replacement.data")
			err = os.WriteFile(replacement, []byte("identical movie"), 0600)
			if err != nil {
				t.Fatal(err)
			}
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
			s.runMovieScan(root)
			current, err := readTestMovieByPath(context.Background(), s.queries, path)
			if err != nil {
				t.Fatal(err)
			}
			if current.ID != original.ID || probe.calls != 2 || invalidations != 1 || s.Status().Updated != 1 {
				t.Fatalf("replacement was missed: %+v probes=%d invalidations=%d", s.Status(), probe.calls, invalidations)
			}
		})
	}
}
