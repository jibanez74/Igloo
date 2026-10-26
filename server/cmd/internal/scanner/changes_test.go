package scanner

import (
	"context"
	"database/sql"
	"errors"
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

// guardFixture is a resolved library root with a database to record its
// mount points in.
func guardFixture(t *testing.T) (string, TxRunner) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, queries := scannertest.OpenDB(t, ":memory:")
	return root, TxRunner{DB: db, Mu: &sync.Mutex{}, Queries: queries}
}

// mountTable answers MountPoints with the given mount points.
func mountTable(points ...string) func() ([]string, error) {
	return func() ([]string, error) { return points, nil }
}

func TestReconciliationGuardKeepsEverythingAfterACatchUpThatFoundNothing(t *testing.T) {
	root, tx := guardFixture(t)
	files := []CatalogFile{{1, filepath.Join(root, "A", "a.mkv")}}
	for _, tc := range []struct {
		changes *Changes
		kept    int
	}{
		{&Changes{CatchUp: true}, 1},
		{nil, 0},
		{&Changes{CatchUp: true, Manual: true}, 0},
		{&Changes{Paths: []string{"A"}}, 0},
	} {
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		kept, err := r.Guard(context.Background(), tx, tc.changes, mountTable())
		if err != nil {
			t.Fatal(err)
		}
		if kept.Files != tc.kept || len(r.Unseen()) != 1-tc.kept {
			t.Fatalf("%+v: kept %+v, unseen %v", tc.changes, kept, r.Unseen())
		}
		if tc.kept > 0 && (kept.Reason != ReasonCleanupSkippedEmpty || kept.MountPoints != nil) {
			t.Fatalf("an empty walk keeps the whole library: %+v", kept)
		}
	}
}

// A disk mounted inside the library that has not come up, as after a power
// cut, leaves an empty mount point. Its missing files are kept by every
// catch-up until it is back, whatever device numbers the disks get, while a
// file deleted from a mounted disk still goes; a library scan an admin asks
// for deletes them and forgets the mount point.
func TestReconciliationGuardKeepsFilesBeneathAMountPointThatIsNotMounted(t *testing.T) {
	root, tx := guardFixture(t)
	disk1, disk2 := filepath.Join(root, "disk1"), filepath.Join(root, "disk2")
	files := []CatalogFile{
		{1, filepath.Join(disk1, "seen.mkv")},
		{2, filepath.Join(disk1, "gone.mkv")},
		{3, filepath.Join(disk2, "a.mkv")},
		{4, filepath.Join(disk2, "b.mkv")},
		{5, filepath.Join(root, "gone.mkv")},
	}
	guard := func(changes *Changes, mounts func() ([]string, error)) (Kept, []int64) {
		t.Helper()
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		r.MarkSeen(files[0].Path)
		kept, err := r.Guard(context.Background(), tx, changes, mounts)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, file := range r.Unseen() {
			ids = append(ids, file.ID)
		}
		return kept, ids
	}
	recorded := func() []string {
		t.Helper()
		points, err := tx.Queries.ListLibraryMountPoints(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		return points
	}

	kept, _ := guard(nil, mountTable(disk1, disk2))
	if kept.Files != 0 || !slices.Equal(recorded(), []string{"disk1", "disk2"}) {
		t.Fatalf("a library scan with both disks mounted: kept %+v, recorded %q", kept, recorded())
	}
	for range 2 {
		kept, ids := guard(&Changes{CatchUp: true}, mountTable(disk1))
		if kept.Files != 2 || kept.Reason != ReasonCleanupKeptUnmounted || !slices.Equal(kept.MountPoints, []string{disk2}) {
			t.Fatalf("catch-up with disk2 missing kept %+v", kept)
		}
		if !slices.Equal(ids, []int64{2, 5}) {
			t.Fatalf("cleanup candidates %v, want the files deleted from mounted filesystems", ids)
		}
		if !slices.Equal(recorded(), []string{"disk1", "disk2"}) {
			t.Fatalf("a catch-up forgot a mount point: %q", recorded())
		}
	}
	kept, ids := guard(&Changes{CatchUp: true, Manual: true}, mountTable(disk1))
	if kept.Files != 0 || !slices.Equal(ids, []int64{2, 3, 4, 5}) || !slices.Equal(recorded(), []string{"disk1"}) {
		t.Fatalf("an admin's scan: kept %+v, candidates %v, recorded %q", kept, ids, recorded())
	}
	kept, _ = guard(&Changes{CatchUp: true}, mountTable(disk1))
	if kept.Files != 0 {
		t.Fatalf("a forgotten mount point still kept files: %+v", kept)
	}

	// A run of changes keeps nothing, but records a disk that mounted since,
	// whose first files it may import.
	disk3 := filepath.Join(root, "disk3")
	files = append(files, CatalogFile{6, filepath.Join(disk3, "c.mkv")})
	kept, _ = guard(&Changes{Paths: []string{"disk3"}}, mountTable(disk1, disk3))
	if kept.Files != 0 || !slices.Equal(recorded(), []string{"disk1", "disk3"}) {
		t.Fatalf("a run of changes: kept %+v, recorded %q", kept, recorded())
	}
	kept, _ = guard(&Changes{CatchUp: true}, mountTable(disk1))
	if kept.Files != 1 || !slices.Equal(kept.MountPoints, []string{disk3}) {
		t.Fatalf("a disk a run of changes recorded was not kept: %+v", kept)
	}
}

// A library whose root is itself a mount point keeps every missing file
// while it is not mounted, even when something left in the empty mount point
// is found.
func TestReconciliationGuardKeepsTheFilesOfARootThatIsNotMounted(t *testing.T) {
	root, tx := guardFixture(t)
	files := []CatalogFile{{1, filepath.Join(root, "stray.mkv")}, {2, filepath.Join(root, "A", "a.mkv")}}
	for _, tc := range []struct {
		mounts func() ([]string, error)
		kept   int
	}{{mountTable(root), 0}, {mountTable(), 1}} {
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		r.MarkSeen(files[0].Path)
		kept, err := r.Guard(context.Background(), tx, &Changes{CatchUp: true}, tc.mounts)
		if err != nil {
			t.Fatal(err)
		}
		if kept.Files != tc.kept {
			t.Fatalf("kept %+v, want %d", kept, tc.kept)
		}
	}
}

// A library configured through a symlink records and checks its mount points
// under the resolved root, the paths the mount table holds.
func TestReconciliationGuardResolvesASymlinkedRoot(t *testing.T) {
	resolved, tx := guardFixture(t)
	link := filepath.Join(t.TempDir(), "library")
	err := os.Symlink(resolved, link)
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(resolved, "disk")
	files := []CatalogFile{{1, filepath.Join(link, "seen.mkv")}, {2, filepath.Join(link, "disk", "a.mkv")}}
	for _, tc := range []struct {
		mounts func() ([]string, error)
		kept   int
	}{{mountTable(disk), 0}, {mountTable(), 1}} {
		r, err := NewReconciliation(context.Background(), link, files)
		if err != nil {
			t.Fatal(err)
		}
		r.MarkSeen(files[0].Path)
		kept, err := r.Guard(context.Background(), tx, &Changes{CatchUp: true}, tc.mounts)
		if err != nil {
			t.Fatal(err)
		}
		if kept.Files != tc.kept || len(r.Unseen()) != 1-tc.kept {
			t.Fatalf("kept %+v, want %d; unseen %v", kept, tc.kept, r.Unseen())
		}
	}
}

// A mount table that cannot be read fails the guard before anything is
// recorded or deleted.
func TestReconciliationGuardFailsWithoutAMountTable(t *testing.T) {
	root, tx := guardFixture(t)
	files := []CatalogFile{{1, filepath.Join(root, "seen.mkv")}, {2, filepath.Join(root, "gone.mkv")}}
	r, err := NewReconciliation(context.Background(), root, files)
	if err != nil {
		t.Fatal(err)
	}
	r.MarkSeen(files[0].Path)
	unreadable := errors.New("mount table unreadable")
	_, err = r.Guard(context.Background(), tx, &Changes{CatchUp: true}, func() ([]string, error) { return nil, unreadable })
	if !errors.Is(err, unreadable) {
		t.Fatalf("Guard error = %v", err)
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
		name     string
		requests []func() *Changes
		root     string
		want     string
	}{
		{"root changed while queued", []func() *Changes{manual}, "B", "B:library"},
		{"catch-up takes in a queued scan", []func() *Changes{manual, catchUp}, "A", "A:manual catch-up"},
		{"a scan joins a queued catch-up", []func() *Changes{catchUp, manual}, "A", "A:manual catch-up"},
		{"catch-ups coalesce", []func() *Changes{catchUp, catchUp}, "A", "A:catch-up"},
		{"cleared while queued", []func() *Changes{manual}, "", ""},
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
					kind := "library"
					switch {
					case changes.Guarded():
						kind = "catch-up"
					case changes != nil:
						kind = "manual catch-up"
					}
					completed <- directory + ":" + kind
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
				got := <-completed
				if got != tc.want {
					t.Fatalf("ran %q, want %q", got, tc.want)
				}
			}
			if launcher.queued != nil || launcher.running {
				t.Fatal("launcher retained queued or running state")
			}
		})
	}
}
