package watch

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

var sessionTestTiming = timing{
	settle:    50 * time.Millisecond,
	maxBatch:  2 * time.Second,
	writeIdle: time.Second,
	quiet:     100 * time.Millisecond,
	retry:     100 * time.Millisecond,
}

var sessionTestPolling = pollTiming{
	interval: 100 * time.Millisecond,
	settle:   50 * time.Millisecond,
	recheck:  100 * time.Millisecond,
	quiet:    200 * time.Millisecond,
}

// fakeScanner records every run the watcher starts and answers with scripted
// statuses, StartStarted once the script runs out.
type fakeScanner struct {
	calls  chan scanner.Changes
	mu     sync.Mutex
	script []scanner.StartStatus
}

func newFakeScanner() *fakeScanner {
	return &fakeScanner{calls: make(chan scanner.Changes, 100)}
}

func (f *fakeScanner) start(changes scanner.Changes) scanner.StartResult {
	f.mu.Lock()
	status := scanner.StartStarted
	if len(f.script) > 0 {
		status, f.script = f.script[0], f.script[1:]
	}
	f.mu.Unlock()
	f.calls <- changes
	return scanner.StartResult{Status: status}
}

// next waits for the next run.
func (f *fakeScanner) next(t *testing.T) scanner.Changes {
	t.Helper()
	select {
	case changes := <-f.calls:
		return changes
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher started no run")
		return scanner.Changes{}
	}
}

// nextChanges skips catch-up runs and returns the next run of changes.
func (f *fakeScanner) nextChanges(t *testing.T) scanner.Changes {
	t.Helper()
	for {
		changes := f.next(t)
		if !changes.CatchUp {
			return changes
		}
	}
}

// quiet fails if a run starts within wait.
func (f *fakeScanner) quiet(t *testing.T, wait time.Duration) {
	t.Helper()
	select {
	case changes := <-f.calls:
		t.Fatalf("unexpected run: %+v", changes)
	case <-time.After(wait):
	}
}

type watcherFixture struct {
	watcher *Watcher
	scanner *fakeScanner
	root    string
	wait    *sync.WaitGroup
	cancel  context.CancelFunc
}

func newWatcherFixture(t *testing.T, configure func(*Library)) *watcherFixture {
	t.Helper()
	root := t.TempDir()
	fake := newFakeScanner()
	lib := Library{
		Name:         "movies",
		Directory:    func() sql.NullString { return sql.NullString{String: root, Valid: true} },
		Extensions:   map[string]bool{"mkv": true},
		StartChanges: fake.start,
	}
	if configure != nil {
		configure(&lib)
	}
	ctx, cancel := context.WithCancel(context.Background())
	wait := &sync.WaitGroup{}
	w := New(Dependencies{Context: ctx, Wait: wait, Logger: &scannertest.Logger{}, Libraries: []Library{lib}})
	w.timing, w.polling = sessionTestTiming, sessionTestPolling
	fixture := &watcherFixture{watcher: w, scanner: fake, root: root, wait: wait, cancel: cancel}
	t.Cleanup(func() {
		w.Sync(false)
		cancel()
		scannertest.WaitForGroup(t, wait, 5*time.Second, "watcher goroutines to exit")
	})
	return fixture
}

func (f *watcherFixture) path(parts ...string) string {
	return filepath.Join(append([]string{f.root}, parts...)...)
}

func sortedPaths(changes scanner.Changes) []string {
	paths := slices.Clone(changes.Paths)
	slices.Sort(paths)
	return paths
}

func running(w *Watcher) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.active()
}

func inotifyDescriptors(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skip("no /proc/self/fd")
	}
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && strings.Contains(target, "inotify") {
			count++
		}
	}
	return count
}

func TestWatcherOffMeansNothingRuns(t *testing.T) {
	before := inotifyDescriptors(t)
	fixture := newWatcherFixture(t, nil)
	fixture.watcher.Sync(false)
	if running(fixture.watcher) || inotifyDescriptors(t) != before {
		t.Fatal("a watcher that was never enabled must hold nothing")
	}

	fixture.watcher.Sync(true)
	catchUp := fixture.scanner.next(t)
	if !catchUp.CatchUp {
		t.Fatalf("turning the watcher on must catch up first, got %+v", catchUp)
	}
	if !running(fixture.watcher) || inotifyDescriptors(t) != before+1 {
		t.Fatalf("running watcher: inotify descriptors %d, want %d", inotifyDescriptors(t), before+1)
	}

	fixture.watcher.Sync(false)
	if running(fixture.watcher) || inotifyDescriptors(t) != before {
		t.Fatal("turning the watcher off must close its inotify instance before returning")
	}
	scannertest.WriteFile(t, fixture.path("Ignored (2020)", "ignored.mkv"), "x")
	fixture.scanner.quiet(t, 300*time.Millisecond)
}

func TestWatcherAppliesCreatesWritesRenamesAndRemovals(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scannertest.WriteFile(t, fixture.path("Old (2001)", "old.mkv"), "old")
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("expected the catch-up first")
	}

	// A copy into a new directory: the directory is watched, then its file.
	scannertest.WriteFile(t, fixture.path("New (2020)", "new.mkv"), "new")
	scannertest.WriteFile(t, fixture.path("New (2020)", "new.srt"), "subtitle")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/new.mkv"}) {
		t.Fatalf("copy: %q", got)
	}

	// A file already present in a directory moved in is applied too.
	outside := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(outside, "Moved (2019)", "moved.mkv"), "moved")
	err := os.Rename(filepath.Join(outside, "Moved (2019)"), fixture.path("Moved (2019)"))
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Moved (2019)/moved.mkv"}) {
		t.Fatalf("move in: %q", got)
	}

	// A rename dispatches both names in one run.
	err = os.Rename(fixture.path("New (2020)", "new.mkv"), fixture.path("New (2020)", "renamed.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/new.mkv", "New (2020)/renamed.mkv"}) {
		t.Fatalf("rename: %q", got)
	}

	// Removing a directory removes everything beneath it, and its watches.
	err = os.RemoveAll(fixture.path("Old (2001)"))
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Contains(got, "Old (2001)") {
		t.Fatalf("removal: %q", got)
	}

	// Writing into a watched file is applied once the writer closes it.
	handle, err := os.OpenFile(fixture.path("Moved (2019)", "moved.mkv"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handle.WriteString(" more")
	if err != nil {
		t.Fatal(err)
	}
	fixture.scanner.quiet(t, 400*time.Millisecond)
	err = handle.Close()
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Moved (2019)/moved.mkv"}) {
		t.Fatalf("rewrite: %q", got)
	}
}

func TestWatcherRetriesABusyScannerAndDeferredFiles(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	fixture.scanner.script = []scanner.StartStatus{scanner.StartAlreadyRunning, scanner.StartStarted, scanner.StartAlreadyRunning}
	fixture.watcher.Sync(true)
	// The busy catch-up is retried.
	if !fixture.scanner.next(t).CatchUp || !fixture.scanner.next(t).CatchUp {
		t.Fatal("a refused catch-up must be retried")
	}

	scannertest.WriteFile(t, fixture.path("A (2000)", "a.mkv"), "a")
	first := fixture.scanner.nextChanges(t)
	retried := fixture.scanner.nextChanges(t)
	if !slices.Equal(first.Paths, retried.Paths) {
		t.Fatalf("a refused batch must come back unchanged: %q then %q", first.Paths, retried.Paths)
	}

	// A file the scanner deferred is dispatched again once eligible.
	retried.OnDeferred("A (2000)/a.mkv", time.Now().Add(200*time.Millisecond))
	again := fixture.scanner.nextChanges(t)
	if !slices.Equal(again.Paths, []string{"A (2000)/a.mkv"}) {
		t.Fatalf("deferred file: %q", again.Paths)
	}
}

func TestWatcherKeepsShowsToTheDirectoriesTheScannerWalks(t *testing.T) {
	fixture := newWatcherFixture(t, func(lib *Library) {
		lib.Name = "shows"
		lib.AllowDirectory = func(root, path string) bool {
			rel, _ := filepath.Rel(root, path)
			parts := strings.Split(rel, string(filepath.Separator))
			return len(parts) == 1 || len(parts) == 2 && strings.HasPrefix(parts[1], "Season ")
		}
	})
	fixture.watcher.Sync(true)
	fixture.scanner.next(t)
	scannertest.WriteFile(t, fixture.path("Show", "Season 1", "Extras", "x.mkv"), "x")
	fixture.scanner.quiet(t, 400*time.Millisecond)
	scannertest.WriteFile(t, fixture.path("Show", "Season 1", "S01E01.mkv"), "e")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Show/Season 1/S01E01.mkv"}) {
		t.Fatalf("show changes: %q", got)
	}
}

func TestWatcherPausesALibraryWhoseRootDisappears(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scannertest.WriteFile(t, fixture.path("A (2000)", "a.mkv"), "a")
	fixture.watcher.Sync(true)
	fixture.scanner.next(t)

	err := os.RemoveAll(fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	// Nothing is dispatched for a library that is gone, not even removals.
	fixture.scanner.quiet(t, 500*time.Millisecond)

	// When it returns, the watcher arms it again and walks it once.
	scannertest.WriteFile(t, fixture.path("B (2001)", "b.mkv"), "b")
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a library that came back must be walked once")
	}
	scannertest.WriteFile(t, fixture.path("C (2002)", "c.mkv"), "c")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"C (2002)/c.mkv"}) {
		t.Fatalf("after re-arming: %q", got)
	}
}

func TestWatcherPollsWithoutEvents(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return nil, errNotifierUnsupported }
	scannertest.WriteFile(t, fixture.path("Old (2001)", "old.mkv"), "old")
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a polled library catches up once its snapshot is taken")
	}
	scannertest.WriteFile(t, fixture.path("New (2020)", "new.mkv"), "new")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/new.mkv"}) {
		t.Fatalf("polled create: %q", got)
	}
	err := os.Remove(fixture.path("Old (2001)", "old.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Old (2001)/old.mkv"}) {
		t.Fatalf("polled removal: %q", got)
	}
}

func TestWatcherReArmsWhenTheRootSettingChanges(t *testing.T) {
	var mu sync.Mutex
	first := t.TempDir()
	current := first
	fixture := newWatcherFixture(t, func(lib *Library) {
		lib.Directory = func() sql.NullString {
			mu.Lock()
			defer mu.Unlock()
			return sql.NullString{String: current, Valid: true}
		}
	})
	fixture.watcher.Sync(true)
	fixture.scanner.next(t)

	second := t.TempDir()
	mu.Lock()
	current = second
	mu.Unlock()
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a new root must be walked once")
	}
	scannertest.WriteFile(t, filepath.Join(first, "Stale (2000)", "stale.mkv"), "x")
	fixture.scanner.quiet(t, 300*time.Millisecond)
	scannertest.WriteFile(t, filepath.Join(second, "Fresh (2001)", "fresh.mkv"), "x")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Fresh (2001)/fresh.mkv"}) {
		t.Fatalf("after the root changed: %q", got)
	}
}

// scriptedNotifier hands out watch descriptors without a kernel instance, so a
// test can fail additions or inject events.
type scriptedNotifier struct {
	mu     sync.Mutex
	next   int32
	addErr error
	out    chan rawEvent
	once   sync.Once
}

func newScriptedNotifier() *scriptedNotifier {
	return &scriptedNotifier{out: make(chan rawEvent, 16)}
}

func (n *scriptedNotifier) add(string) (int32, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.addErr != nil {
		return 0, n.addErr
	}
	n.next++
	return n.next, nil
}

func (n *scriptedNotifier) remove(int32) error      { return nil }
func (n *scriptedNotifier) events() <-chan rawEvent { return n.out }
func (n *scriptedNotifier) err() error              { return nil }
func (n *scriptedNotifier) close() error            { return nil }

func TestWatcherCatchesUpAfterLostEvents(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scripted := newScriptedNotifier()
	fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return scripted, nil }
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("expected the first catch-up")
	}
	scripted.out <- rawEvent{wd: -1, op: opOverflow}
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a queue overflow must be followed by a catch-up")
	}
}

func TestWatcherPollsWhenTheWatchLimitIsReached(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scripted := newScriptedNotifier()
	scripted.addErr = os.NewSyscallError("inotify_add_watch", syscall.ENOSPC)
	fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return scripted, nil }
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("the polled library must still catch up")
	}
	scannertest.WriteFile(t, fixture.path("New (2020)", "new.mkv"), "new")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/new.mkv"}) {
		t.Fatalf("polled create after the watch limit: %q", got)
	}
}

func TestPollerReportsNoRemovalsFromAReplacedDirectory(t *testing.T) {
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "Share", "a.mkv"), "a")
	p := &poller{root: root, extensions: map[string]bool{"mkv": true}, timing: sessionTestPolling, now: time.Now, layout: layout{root: root}, offline: true}
	if report := p.poll(); !report.online {
		t.Fatalf("first poll: %+v", report)
	}
	// The directory is replaced by an empty one, as an unmounted share's
	// mount point would be: its entries are gone but nothing proves they
	// were deleted.
	// The empty directory exists before the old one goes, so it cannot reuse
	// the old inode number.
	err := os.Mkdir(filepath.Join(root, "Share.empty"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll(filepath.Join(root, "Share"))
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(filepath.Join(root, "Share.empty"), filepath.Join(root, "Share"))
	if err != nil {
		t.Fatal(err)
	}
	report := p.poll()
	for _, change := range report.changes {
		if change.kind == changeRemoved && change.rel == "Share/a.mkv" {
			t.Fatalf("a replaced directory reported a removal: %+v", report.changes)
		}
	}
}
