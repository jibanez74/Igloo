package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
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
		// A sibling that sorts between a directory and its descendant must
		// not keep the descendant.
		"A/c.mkv",
		"A b",
		"A",
	}}
	got := changes.Resolve(root)
	want := []string{
		"/library/A",
		"/library/A b",
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
	onDeferred := func(string, time.Time) {}
	catchUp := &Changes{CatchUp: true, OnDeferred: onDeferred}
	scoped := &Changes{Paths: []string{"a.mkv"}, OnDeferred: onDeferred}
	startup := &Changes{CatchUp: true}
	const libraryReason = "retried on the next scan"
	if scoped.QuietPeriod() != ChangeQuietPeriod || !scoped.Scoped() || scoped.Scope() != ScopeChanges || scoped.DeferredReason(libraryReason) != ReasonFileDeferredWatched {
		t.Fatal("a run of changes must use ChangeQuietPeriod, be scoped, and leave deferred files to the watcher")
	}
	if catchUp.QuietPeriod() != FileQuietPeriod || catchUp.Scoped() || catchUp.Scope() != ScopeLibrary || catchUp.DeferredReason(libraryReason) != ReasonFileDeferredWatched {
		t.Fatal("a catch-up run must keep FileQuietPeriod and walk everything, and the watcher retries what it defers")
	}
	if startup.HandsBackDeferred() || startup.DeferredReason(libraryReason) != libraryReason {
		t.Fatal("a startup catch-up has no watcher to hand its deferred files to")
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

func TestReconciliationGuardKeepsOnlyACatchUpFromAnEmptyWalk(t *testing.T) {
	root := t.TempDir()
	files := []CatalogFile{{ID: 1, Path: filepath.Join(root, "A", "a.mkv")}}
	newReconciliation := func(files []CatalogFile) *Reconciliation {
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	catchUp := &Changes{CatchUp: true}
	r := newReconciliation(files)
	if kept, reason := r.Guard(catchUp); kept != 1 || reason != ReasonCleanupSkippedEmpty || len(r.Unseen()) != 0 {
		t.Fatalf("a catch-up that saw nothing under a populated catalog kept %d (%q), unseen %v", kept, reason, r.Unseen())
	}
	for _, changes := range []*Changes{nil, {Paths: []string{"A"}}} {
		r := newReconciliation(files)
		if kept, reason := r.Guard(changes); kept != 0 || reason != "" || len(r.Unseen()) != 1 {
			t.Fatalf("only a catch-up keeps files: %+v kept %d (%q)", changes, kept, reason)
		}
	}
	if kept, reason := newReconciliation(nil).Guard(catchUp); kept != 0 || reason != "" {
		t.Fatalf("an empty catalog has nothing to keep: %d (%q)", kept, reason)
	}
}

// A disk mounted inside the library that has not come up leaves an empty
// mount point on the root's filesystem: its files are missing, and the walk
// found nothing else of the device they recorded.
func TestReconciliationGuardKeepsFilesOfAFilesystemTheWalkFoundNothingOf(t *testing.T) {
	root := t.TempDir()
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	rootDevice := FingerprintOf(info).Device
	path := func(name string) string { return filepath.Join(root, name) }
	files := []CatalogFile{
		{ID: 1, Path: path("disk1/seen.mkv"), Device: "remounted"},
		{ID: 2, Path: path("disk1/gone.mkv"), Device: "remounted"},
		{ID: 3, Path: path("disk2/a.mkv"), Device: "unmounted"},
		{ID: 4, Path: path("disk2/b.mkv"), Device: "unmounted"},
		{ID: 5, Path: path("local.mkv"), Device: rootDevice},
		{ID: 6, Path: path("legacy.mkv")},
	}
	r, err := NewReconciliation(context.Background(), root, files)
	if err != nil {
		t.Fatal(err)
	}
	r.MarkSeen(files[0].Path)
	kept, reason := r.Guard(&Changes{CatchUp: true})
	if kept != 2 || reason != ReasonCleanupKeptUnmounted {
		t.Fatalf("kept %d (%q), want the two files of the unmounted disk", kept, reason)
	}
	var ids []int64
	for _, file := range r.Unseen() {
		ids = append(ids, file.ID)
	}
	// A device the walk saw a cataloged file of is mounted, even under a
	// number the root does not have; the root's own device is mounted; a file
	// without a recorded device gives no evidence either way.
	if want := []int64{2, 5, 6}; !slices.Equal(ids, want) {
		t.Fatalf("cleanup candidates %v, want %v", ids, want)
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

	result := launcher.Start(fixedDirectory(directory), scoped, begin, func(directory string, changes *Changes) {
		<-release
		record("changes")(directory, changes)
	})
	if result.Status != StartStarted {
		t.Fatalf("changes status = %d", result.Status)
	}
	// A second run of changes is refused; the watcher retries it.
	result = launcher.Start(fixedDirectory(directory), scoped, begin, record("second changes"))
	if result.Status != StartAlreadyRunning {
		t.Fatalf("second changes status = %d, want already running", result.Status)
	}
	// A library scan is queued behind it, a repeat request joins the queue,
	// and a catch-up takes its place.
	queued := []struct {
		changes *Changes
		name    string
	}{{nil, "library"}, {nil, "repeat"}, {catchUp, "catch-up"}, {nil, "after catch-up"}}
	for _, request := range queued {
		result = launcher.Start(fixedDirectory(directory), request.changes, begin, record(request.name))
		if result.Status != StartQueued {
			t.Fatalf("%s status = %d, want queued", request.name, result.Status)
		}
	}
	close(release)
	scans.Wait()
	if strings.Join(order, ",") != "changes,catch-up" || !reflect.DeepEqual(scopes, []ScanScope{ScopeChanges, ScopeLibrary}) {
		t.Fatalf("order = %q, scopes = %q", order, scopes)
	}

	// While a library scan or a catch-up runs, every start is refused rather
	// than queued.
	for _, running := range []*Changes{nil, catchUp} {
		block := make(chan struct{})
		launcher.Start(fixedDirectory(directory), running, func(ScanScope) {}, func(string, *Changes) { <-block })
		for _, changes := range []*Changes{nil, catchUp, scoped} {
			status := launcher.Start(fixedDirectory(directory), changes, func(ScanScope) {}, func(string, *Changes) {}).Status
			if status != StartAlreadyRunning {
				t.Fatalf("during %+v: start %+v status %d, want already running", running, changes, status)
			}
		}
		close(block)
		scans.Wait()
	}
	result = launcher.Start(fixedDirectory(directory), scoped, func(ScanScope) {}, func(string, *Changes) {})
	scans.Wait()
	if result.Status != StartStarted {
		t.Fatalf("after the queue drained: %d", result.Status)
	}
	if launcher.Start(fixedDirectory(sql.NullString{}), scoped, func(ScanScope) {}, func(string, *Changes) {}).Status != StartNotConfigured {
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

func fixedDirectory(directory sql.NullString) func() sql.NullString {
	return func() sql.NullString { return directory }
}

// A queued library scan walks the root configured when it launches, and a
// catch-up queued with it keeps its deferral hand-back and empty-root guard.
func TestLauncherQueuedScanReadsTheRootAtLaunch(t *testing.T) {
	manual := func() *Changes { return nil }
	catchUp := func() *Changes { return &Changes{CatchUp: true} }
	for _, tc := range []struct {
		name      string
		requests  []func() *Changes
		root      string
		want      string
		wantCatch bool
	}{
		{"root changed while queued", []func() *Changes{manual}, "B", "B", false},
		{"catch-up replaces a queued scan", []func() *Changes{manual, catchUp}, "A", "A", true},
		{"a scan joins a queued catch-up", []func() *Changes{catchUp, manual}, "A", "A", true},
		{"cleared while queued", []func() *Changes{manual}, "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var scans sync.WaitGroup
			launcher := Launcher{Wait: &scans}
			var mu sync.Mutex
			root := "A"
			directory := func() sql.NullString {
				mu.Lock()
				defer mu.Unlock()
				return sql.NullString{String: root, Valid: root != ""}
			}
			release := make(chan struct{})
			started := make(chan struct{})
			launcher.Start(directory, &Changes{Paths: []string{"file.mkv"}}, func(ScanScope) {}, func(string, *Changes) {
				close(started)
				<-release
			})
			<-started
			completed := make(chan string, len(tc.requests))
			for _, request := range tc.requests {
				result := launcher.Start(directory, request(), func(ScanScope) {}, func(directory string, changes *Changes) {
					completed <- fmt.Sprintf("%s:%v", directory, changes != nil && changes.CatchUp)
				})
				if result.Status != StartQueued || result.Directory != "A" {
					t.Fatalf("request: %+v", result)
				}
			}
			mu.Lock()
			root = tc.root
			mu.Unlock()
			close(release)
			scans.Wait()
			if tc.want == "" {
				if len(completed) != 0 {
					t.Fatalf("a scan queued for a cleared library ran: %s", <-completed)
				}
			} else {
				if len(completed) != 1 {
					t.Fatalf("%d queued scans ran, want one", len(completed))
				}
				got, want := <-completed, fmt.Sprintf("%s:%v", tc.want, tc.wantCatch)
				if got != want {
					t.Fatalf("ran %q, want %q", got, want)
				}
			}
			if launcher.queued != nil || launcher.running {
				t.Fatal("launcher retained queued or running state")
			}
		})
	}
}
