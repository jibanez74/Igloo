package scanner

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/scanner/scannertest"
)

func TestChangesResolve(t *testing.T) {
	root := "/library"
	changes := Changes{Paths: []string{
		"Movie (2020)/movie.mkv",
		"Movie (2020)",
		"Other/./b.mkv",
		"Other/a.mkv",
		"Other/a.mkv",
		"",
		".",
		"..",
		"../outside.mkv",
		"Other/../../outside.mkv",
		"/etc/passwd",
		"Movies (2021)/m.mkv",
	}}
	got := changes.Resolve(root)
	want := []string{
		"/library/Movie (2020)",
		"/library/Movies (2021)/m.mkv",
		"/library/Other/a.mkv",
		"/library/Other/b.mkv",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
}

func TestChangesQuietPeriodAndDeferred(t *testing.T) {
	var library *Changes
	catchUp := &Changes{CatchUp: true}
	scoped := &Changes{Paths: []string{"a.mkv"}}
	const libraryReason = "retried on the next scan"
	if scoped.QuietPeriod() != ChangeQuietPeriod || !scoped.Scoped() || scoped.Scope() != ScopeChanges || scoped.DeferredReason(libraryReason) != ReasonFileDeferredWatched {
		t.Fatal("a run of changes must use ChangeQuietPeriod, be scoped, and leave deferred files to the watcher")
	}
	if catchUp.QuietPeriod() != FileQuietPeriod || catchUp.Scoped() || catchUp.Scope() != ScopeLibrary || catchUp.DeferredReason(libraryReason) != ReasonFileDeferredWatched {
		t.Fatal("a catch-up run must keep FileQuietPeriod and walk everything, and the watcher retries what it defers")
	}
	if library.QuietPeriod() != FileQuietPeriod || library.Scoped() || library.Scope() != ScopeLibrary || library.DeferredReason(libraryReason) != libraryReason || library.Resolve("/library") != nil {
		t.Fatal("a library scan keeps today's behavior")
	}
	library.Deferred("/library", "/library/a.mkv", time.Time{})
	var got []string
	eligible := time.Unix(100, 0)
	changes := &Changes{OnDeferred: func(path string, eligibleAt time.Time) {
		if !eligibleAt.Equal(eligible) {
			t.Errorf("eligibleAt = %v", eligibleAt)
		}
		got = append(got, path)
	}}
	changes.Deferred("/library", "/library/Show/Season 1/e1.mkv", eligible)
	(&Changes{}).Deferred("/library", "/library/ignored.mkv", eligible)
	if !reflect.DeepEqual(got, []string{"Show/Season 1/e1.mkv"}) {
		t.Fatalf("deferred = %q", got)
	}
}

func walkChangesPaths(t *testing.T, root string, paths []string, filter func(string) bool) ([]string, []string) {
	t.Helper()
	var files, failures []string
	err := walkChanges(context.Background(), root, paths, map[string]bool{"mkv": true},
		func(path string, _ error) { failures = append(failures, path) },
		func(file ScanFile) error {
			files = append(files, file.Path)
			return nil
		}, filter)
	if err != nil {
		t.Fatalf("walkChanges: %v", err)
	}
	sort.Strings(files)
	return files, failures
}

func TestWalkChangesVisitsOnlyChangedPaths(t *testing.T) {
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "A", "a.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(root, "A", "nested", "b.mkv"), "b")
	scannertest.WriteFile(t, filepath.Join(root, "A", "notes.txt"), "n")
	scannertest.WriteFile(t, filepath.Join(root, "B", "c.mkv"), "c")
	scannertest.WriteFile(t, filepath.Join(root, "B", "skipped.mkv"), "s")
	scannertest.WriteFile(t, filepath.Join(root, "B", "subtitle.srt"), "s")
	err := os.Symlink(filepath.Join(root, "B", "c.mkv"), filepath.Join(root, "link.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(filepath.Join(root, "gone.mkv"), filepath.Join(root, "broken.mkv"))
	if err != nil {
		t.Fatal(err)
	}

	paths := (&Changes{Paths: []string{"A", "B/c.mkv", "B/subtitle.srt", "missing.mkv", "Missing Dir", "link.mkv", "broken.mkv"}}).Resolve(root)
	files, failures := walkChangesPaths(t, root, paths, nil)
	want := []string{
		filepath.Join(root, "A", "a.mkv"),
		filepath.Join(root, "A", "nested", "b.mkv"),
		filepath.Join(root, "B", "c.mkv"),
		filepath.Join(root, "link.mkv"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %q, want %q", files, want)
	}
	// A broken symlink is reported like the full walk reports it.
	if !reflect.DeepEqual(failures, []string{filepath.Join(root, "broken.mkv")}) {
		t.Fatalf("failures = %q", failures)
	}
}

func TestWalkChangesAppliesDirectoryFilterToAncestors(t *testing.T) {
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "Show", "Season 1", "e1.mkv"), "e")
	scannertest.WriteFile(t, filepath.Join(root, "Show", "Season 1", "Extras", "x.mkv"), "x")
	scannertest.WriteFile(t, filepath.Join(root, "Show", "Extras", "y.mkv"), "y")
	// Only Show and Show/Season 1 are reachable, as the show walk allows.
	filter := func(path string) bool {
		relative, _ := filepath.Rel(root, path)
		return relative == "Show" || relative == filepath.Join("Show", "Season 1")
	}
	paths := (&Changes{Paths: []string{
		"Show/Season 1/e1.mkv",
		"Show/Season 1/Extras/x.mkv",
		"Show/Season 1/Extras",
		"Show/Extras",
	}}).Resolve(root)
	files, failures := walkChangesPaths(t, root, paths, filter)
	if !reflect.DeepEqual(files, []string{filepath.Join(root, "Show", "Season 1", "e1.mkv")}) {
		t.Fatalf("files = %q", files)
	}
	if len(failures) != 0 {
		t.Fatalf("failures = %q", failures)
	}

	// A changed show directory is walked with the same filter beneath it.
	files, _ = walkChangesPaths(t, root, (&Changes{Paths: []string{"Show"}}).Resolve(root), filter)
	if !reflect.DeepEqual(files, []string{filepath.Join(root, "Show", "Season 1", "e1.mkv")}) {
		t.Fatalf("directory files = %q", files)
	}
}

func TestWalkChangesStopsOnCancelAndVisitError(t *testing.T) {
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "A", "a.mkv"), "a")
	paths := (&Changes{Paths: []string{"A"}}).Resolve(root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := walkChanges(ctx, root, paths, map[string]bool{"mkv": true}, func(string, error) {}, func(ScanFile) error { return nil }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled walk error = %v", err)
	}

	stop := errors.New("stop")
	err = walkChanges(context.Background(), root, paths, map[string]bool{"mkv": true}, func(string, error) {}, func(ScanFile) error { return stop }, nil)
	if !errors.Is(err, stop) {
		t.Fatalf("visit error = %v, want %v", err, stop)
	}
}

func TestReconciliationRefusesCleanupOnlyForAnEmptyCatchUp(t *testing.T) {
	root := t.TempDir()
	files := []CatalogFile{{1, filepath.Join(root, "A", "a.mkv")}}
	r, err := NewReconciliation(context.Background(), root, files)
	if err != nil {
		t.Fatal(err)
	}
	catchUp := &Changes{CatchUp: true}
	if !r.RefusesCleanup(catchUp) {
		t.Fatal("a catch-up that saw nothing under a populated catalog must not clean up")
	}
	if r.RefusesCleanup(nil) || r.RefusesCleanup(&Changes{Paths: []string{"A"}}) {
		t.Fatal("only a catch-up refuses: a library scan and a run of changes clean up")
	}
	r.MarkSeen(files[0].Path)
	if r.RefusesCleanup(catchUp) {
		t.Fatal("a catch-up that saw a file cleans up")
	}

	empty, err := NewReconciliation(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.RefusesCleanup(catchUp) {
		t.Fatal("an empty catalog has nothing to protect")
	}
}

func TestWalkRunWalksWhatTheRunCovers(t *testing.T) {
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "A", "a.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(root, "B", "b.mkv"), "b")
	walk := func(changes *Changes) []string {
		var files []string
		err := WalkRun(context.Background(), root, changes, changes.Resolve(root), map[string]bool{"mkv": true},
			func(string, error) {}, func(file ScanFile) error {
				files = append(files, file.Path)
				return nil
			}, nil)
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(files)
		return files
	}
	all := []string{filepath.Join(root, "A", "a.mkv"), filepath.Join(root, "B", "b.mkv")}
	for name, changes := range map[string]*Changes{"library scan": nil, "catch-up": {CatchUp: true}} {
		if got := walk(changes); !reflect.DeepEqual(got, all) {
			t.Errorf("%s walked %q, want %q", name, got, all)
		}
	}
	if got := walk(&Changes{Paths: []string{"B"}}); !reflect.DeepEqual(got, all[1:]) {
		t.Errorf("run of changes walked %q", got)
	}
	if got := walk(&Changes{Paths: []string{".."}}); len(got) != 0 {
		t.Errorf("a run whose every path was dropped must walk nothing, walked %q", got)
	}
}

func TestInspectHonorsTheRunQuietPeriod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	scannertest.WriteFile(t, path, "media")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Ten seconds after the write: inside FileQuietPeriod, past ChangeQuietPeriod.
	now := func() time.Time { return info.ModTime().Add(10 * time.Second) }
	library, err := InspectFileMetadata(context.Background(), path, nil, now, FileQuietPeriod)
	if err != nil {
		t.Fatal(err)
	}
	defer library.Close()
	if library.Outcome != FileDeferred {
		t.Fatalf("library scan outcome = %v, want deferred", library.Outcome)
	}
	change, err := InspectFileMetadata(context.Background(), path, nil, now, ChangeQuietPeriod)
	if err != nil {
		t.Fatal(err)
	}
	defer change.Close()
	if change.Outcome != FileNeedsProcessing {
		t.Fatalf("change run outcome = %v, want needs processing", change.Outcome)
	}
}

func TestLauncherQueuesALibraryScanBehindChanges(t *testing.T) {
	var scans sync.WaitGroup
	launcher := Launcher{Wait: &scans}
	directory := sql.NullString{String: "/library", Valid: true}
	scoped := &Changes{Paths: []string{"a.mkv"}}
	catchUp := &Changes{CatchUp: true}
	release := make(chan struct{})
	var mu sync.Mutex
	var order []string
	record := func(name string) func(string, *Changes) {
		return func(string, *Changes) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	var scopes []ScanScope
	begin := func(scope ScanScope) { scopes = append(scopes, scope) }

	result := launcher.Start(directory, scoped, begin, func(directory string, changes *Changes) {
		<-release
		record("changes")(directory, changes)
	})
	if result.Status != StartStarted {
		t.Fatalf("changes status = %d", result.Status)
	}
	// A second run of changes is refused; the watcher retries it.
	result = launcher.Start(directory, scoped, begin, record("second changes"))
	if result.Status != StartAlreadyRunning {
		t.Fatalf("second changes status = %d, want already running", result.Status)
	}
	// A library scan is queued behind it, and a repeat request or a catch-up
	// joins the queue.
	result = launcher.Start(directory, nil, begin, record("library"))
	if result.Status != StartQueued {
		t.Fatalf("library status = %d, want queued", result.Status)
	}
	for _, changes := range []*Changes{nil, catchUp} {
		result = launcher.Start(directory, changes, begin, record("duplicate"))
		if result.Status != StartQueued {
			t.Fatalf("repeat status = %d, want queued", result.Status)
		}
	}
	close(release)
	scans.Wait()
	if strings.Join(order, ",") != "changes,library" || !reflect.DeepEqual(scopes, []ScanScope{ScopeChanges, ScopeLibrary}) {
		t.Fatalf("order = %q, scopes = %q", order, scopes)
	}

	// While a library scan or a catch-up runs, every start is refused rather
	// than queued.
	for _, running := range []*Changes{nil, catchUp} {
		block := make(chan struct{})
		launcher.Start(directory, running, func(ScanScope) {}, func(string, *Changes) { <-block })
		for _, changes := range []*Changes{nil, catchUp, scoped} {
			status := launcher.Start(directory, changes, func(ScanScope) {}, func(string, *Changes) {}).Status
			if status != StartAlreadyRunning {
				t.Fatalf("during %+v: start %+v status %d, want already running", running, changes, status)
			}
		}
		close(block)
		scans.Wait()
	}
	result = launcher.Start(directory, scoped, func(ScanScope) {}, func(string, *Changes) {})
	scans.Wait()
	if result.Status != StartStarted {
		t.Fatalf("after the queue drained: %d", result.Status)
	}
	if launcher.Start(sql.NullString{}, scoped, func(ScanScope) {}, func(string, *Changes) {}).Status != StartNotConfigured {
		t.Fatal("an unconfigured library must not start changes")
	}
}

func TestChildBoundsCoverExactlyThePathsBeneath(t *testing.T) {
	lower, upper := ChildBounds("/library/Movie (2020)")
	within := func(path string) bool { return path >= lower && path < upper }
	for _, path := range []string{"/library/Movie (2020)/m.mkv", "/library/Movie (2020)/Extras/x.mkv"} {
		if !within(path) {
			t.Errorf("%q should be beneath", path)
		}
	}
	for _, path := range []string{"/library/Movie (2020)", "/library/Movie (2020) Extended.mkv", "/library/Movie (2020)0", "/library/Movie (2020)-x/m.mkv", "/library/Other/m.mkv"} {
		if within(path) {
			t.Errorf("%q should not be beneath", path)
		}
	}
}

func TestPathWithin(t *testing.T) {
	tests := []struct {
		parent, path string
		want         bool
	}{
		{"/library", "/library", true},
		{"/library", "/library/a.mkv", true},
		{"/library", "/library2/a.mkv", false},
		{"/library", "/", false},
		{"/", "/library/a.mkv", true},
		{"/", "/", true},
	}
	for _, tc := range tests {
		if got := PathWithin(tc.parent, tc.path); got != tc.want {
			t.Errorf("PathWithin(%q, %q) = %t, want %t", tc.parent, tc.path, got, tc.want)
		}
	}
}
