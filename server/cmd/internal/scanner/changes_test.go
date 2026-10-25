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
	if (Changes{}).QuietPeriod() != ChangeQuietPeriod {
		t.Fatal("a run of changes must use ChangeQuietPeriod")
	}
	if (Changes{CatchUp: true}).QuietPeriod() != FileQuietPeriod {
		t.Fatal("a catch-up run must keep FileQuietPeriod")
	}
	var got []string
	eligible := time.Unix(100, 0)
	changes := Changes{OnDeferred: func(path string, eligibleAt time.Time) {
		if !eligibleAt.Equal(eligible) {
			t.Errorf("eligibleAt = %v", eligibleAt)
		}
		got = append(got, path)
	}}
	changes.Deferred("/library", "/library/Show/Season 1/e1.mkv", eligible)
	Changes{}.Deferred("/library", "/library/ignored.mkv", eligible)
	if !reflect.DeepEqual(got, []string{"Show/Season 1/e1.mkv"}) {
		t.Fatalf("deferred = %q", got)
	}
}

func walkChangesPaths(t *testing.T, root string, paths []string, filter func(string) bool) ([]string, []string) {
	t.Helper()
	var files, failures []string
	err := WalkChanges(context.Background(), root, paths, map[string]bool{"mkv": true},
		func(path string, _ error) { failures = append(failures, path) },
		func(file ScanFile) error {
			files = append(files, file.Path)
			return nil
		}, filter)
	if err != nil {
		t.Fatalf("WalkChanges: %v", err)
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

	paths := Changes{Paths: []string{"A", "B/c.mkv", "B/subtitle.srt", "missing.mkv", "Missing Dir", "link.mkv", "broken.mkv"}}.Resolve(root)
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
	paths := Changes{Paths: []string{
		"Show/Season 1/e1.mkv",
		"Show/Season 1/Extras/x.mkv",
		"Show/Season 1/Extras",
		"Show/Extras",
	}}.Resolve(root)
	files, failures := walkChangesPaths(t, root, paths, filter)
	if !reflect.DeepEqual(files, []string{filepath.Join(root, "Show", "Season 1", "e1.mkv")}) {
		t.Fatalf("files = %q", files)
	}
	if len(failures) != 0 {
		t.Fatalf("failures = %q", failures)
	}

	// A changed show directory is walked with the same filter beneath it.
	files, _ = walkChangesPaths(t, root, Changes{Paths: []string{"Show"}}.Resolve(root), filter)
	if !reflect.DeepEqual(files, []string{filepath.Join(root, "Show", "Season 1", "e1.mkv")}) {
		t.Fatalf("directory files = %q", files)
	}
}

func TestWalkChangesStopsOnCancelAndVisitError(t *testing.T) {
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "A", "a.mkv"), "a")
	paths := Changes{Paths: []string{"A"}}.Resolve(root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WalkChanges(ctx, root, paths, map[string]bool{"mkv": true}, func(string, error) {}, func(ScanFile) error { return nil }, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled walk error = %v", err)
	}

	stop := errors.New("stop")
	err = WalkChanges(context.Background(), root, paths, map[string]bool{"mkv": true}, func(string, error) {}, func(ScanFile) error { return stop }, nil)
	if !errors.Is(err, stop) {
		t.Fatalf("visit error = %v, want %v", err, stop)
	}
}

func TestReconciliationLimitKeepsOnlyChangedRows(t *testing.T) {
	root := t.TempDir()
	files := []CatalogFile{
		{1, filepath.Join(root, "A", "a.mkv")},
		{2, filepath.Join(root, "A", "nested", "b.mkv")},
		{3, filepath.Join(root, "AB", "c.mkv")},
		{4, filepath.Join(root, "B", "d.mkv")},
		{5, filepath.Join(root, "e.mkv")},
	}
	r, err := NewReconciliation(context.Background(), root, files)
	if err != nil {
		t.Fatal(err)
	}
	r.Limit(Changes{Paths: []string{"A", "e.mkv"}}.Resolve(root))
	got := r.Unseen()
	want := []CatalogFile{files[0], files[1], files[4]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unseen = %+v, want %+v", got, want)
	}
	if !r.SawNothing() {
		t.Fatal("a walk that saw nothing under a populated catalog must say so")
	}
	r.MarkSeen(files[0].Path)
	if r.SawNothing() {
		t.Fatal("a walk that saw a file must not report SawNothing")
	}

	empty, err := NewReconciliation(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.SawNothing() {
		t.Fatal("an empty catalog has nothing to protect")
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
	release := make(chan struct{})
	var mu sync.Mutex
	var order []string
	record := func(name string) func(string) {
		return func(string) {
			mu.Lock()
			order = append(order, name)
			mu.Unlock()
		}
	}
	begun := 0
	begin := func() { begun++ }

	result := launcher.LaunchChanges(directory, begin, func(string) {
		<-release
		record("changes")("")
	})
	if result.Status != StartStarted {
		t.Fatalf("changes status = %d", result.Status)
	}
	// A second run of changes is refused; the watcher retries it.
	result = launcher.LaunchChanges(directory, begin, record("second changes"))
	if result.Status != StartAlreadyRunning {
		t.Fatalf("second changes status = %d, want already running", result.Status)
	}
	// A library scan is queued behind it, and a repeat request joins the queue.
	result = launcher.Launch(directory, begin, record("library"))
	if result.Status != StartQueued {
		t.Fatalf("library status = %d, want queued", result.Status)
	}
	result = launcher.Launch(directory, begin, record("duplicate library"))
	if result.Status != StartQueued {
		t.Fatalf("repeat library status = %d, want queued", result.Status)
	}
	close(release)
	scans.Wait()
	if strings.Join(order, ",") != "changes,library" || begun != 2 {
		t.Fatalf("order = %q, begun = %d", order, begun)
	}

	// While a library scan runs, both kinds are refused rather than queued.
	block := make(chan struct{})
	launcher.Launch(directory, func() {}, func(string) { <-block })
	library := launcher.Launch(directory, func() {}, func(string) {})
	changes := launcher.LaunchChanges(directory, func() {}, func(string) {})
	close(block)
	scans.Wait()
	if library.Status != StartAlreadyRunning || changes.Status != StartAlreadyRunning {
		t.Fatalf("during a library scan: library %d, changes %d", library.Status, changes.Status)
	}
	result = launcher.LaunchChanges(directory, func() {}, func(string) {})
	scans.Wait()
	if result.Status != StartStarted {
		t.Fatalf("after the queue drained: %d", result.Status)
	}
	if launcher.LaunchChanges(sql.NullString{}, func() {}, func(string) {}).Status != StartNotConfigured {
		t.Fatal("an unconfigured library must not start changes")
	}
}
