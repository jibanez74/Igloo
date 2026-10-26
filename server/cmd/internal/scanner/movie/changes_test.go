package movie

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/tmdb"
)

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
// point. The catch-up keeps its movies, while a movie deleted from a disk that
// is mounted still goes.
func TestMovieCatchUpKeepsTheMoviesOfADiskThatIsNotMounted(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root := t.TempDir()
	kept := filepath.Join(root, "disk1", "A (2001)", "a.mkv")
	deleted := filepath.Join(root, "disk1", "D (2004)", "d.mkv")
	unmounted := []string{filepath.Join(root, "disk2", "B (2002)", "b.mkv"), filepath.Join(root, "disk2", "C (2003)", "c.mkv")}
	for _, path := range append([]string{kept, deleted}, unmounted...) {
		scannertest.WriteFile(t, path, filepath.Base(path))
	}
	s.scan(root)
	// A test cannot mount a filesystem, so disk2's movies are recorded on a
	// device of their own, as they would be.
	_, err := fixture.db.Exec(`UPDATE movie_file_fingerprints SET device = 'disk2'
		WHERE movie_id IN (SELECT id FROM movies WHERE file_path LIKE ?)`, filepath.Join(root, "disk2")+"/%")
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{filepath.Join(root, "disk2"), filepath.Dir(deleted)} {
		err = os.RemoveAll(directory)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = os.Mkdir(filepath.Join(root, "disk2"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if got, want := catalogPaths(t, s), append([]string{kept}, unmounted...); !slices.Equal(got, want) {
		t.Fatalf("after the catch-up: %q, want %q", got, want)
	}
	if status.Deleted != 1 || status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupKeptUnmounted {
		t.Fatalf("deleted=%d issues=%+v", status.Deleted, status.Issues)
	}

	// A library scan an admin asked for deletes them, for a disk removed for
	// good.
	s.scan(root)
	if got := catalogPaths(t, s); !slices.Equal(got, []string{kept}) {
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
	for _, table := range []string{"keyframe_indexes", "remux_safety_verdicts"} {
		if scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table) != 0 {
			t.Fatalf("replacement retained %s", table)
		}
	}
}
