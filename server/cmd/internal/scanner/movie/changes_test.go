package movie

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
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

func TestMovieReprobeKeepsTheTmdbMissBackoff(t *testing.T) {
	fixture := setupMovieScanner(t)
	s := fixture.scanner
	s.ffprobe = &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "Unknown (1999)", "unknown.mkv"), "unknown")
	s.scan(root)
	_, err := s.tx.DB.Exec("UPDATE movie_tmdb_retries SET attempts = 3, last_attempt_at = 1700000000")
	if err != nil {
		t.Fatal(err)
	}

	// A rewrite keeps the name the search is derived from, so the backoff
	// earned by earlier misses stands.
	scannertest.WriteFile(t, path, "unknown, remuxed")
	s.scanChanges(root, scanner.Changes{Paths: []string{"Unknown (1999)/unknown.mkv"}})
	if status := s.Status(); status.Updated != 1 {
		t.Fatalf("counts = %+v", status.Counts)
	}
	var attempts int
	var lastAttempt int64
	err = s.tx.DB.QueryRow("SELECT attempts, last_attempt_at FROM movie_tmdb_retries").Scan(&attempts, &lastAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 || lastAttempt != 1700000000 {
		t.Fatalf("backoff reset to attempts=%d last=%d", attempts, lastAttempt)
	}
}
