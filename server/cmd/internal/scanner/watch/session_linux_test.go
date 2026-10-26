package watch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	probe:     time.Second,
}

var sessionTestPolling = pollTiming{
	interval: 100 * time.Millisecond,
	settle:   50 * time.Millisecond,
	recheck:  100 * time.Millisecond,
	quiet:    200 * time.Millisecond,
	holdCap:  5 * time.Second,
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

func TestWatcherOffMeansNothingRuns(t *testing.T) {
	before := scannertest.InotifyInstances(t)
	fixture := newWatcherFixture(t, nil)
	fixture.watcher.Sync(false)
	if running(fixture.watcher) || scannertest.InotifyInstances(t) != before {
		t.Fatal("a watcher that was never enabled must hold nothing")
	}

	fixture.watcher.Sync(true)
	catchUp := fixture.scanner.next(t)
	if !catchUp.CatchUp {
		t.Fatalf("turning the watcher on must catch up first, got %+v", catchUp)
	}
	if !running(fixture.watcher) || scannertest.InotifyInstances(t) != before+1 {
		t.Fatalf("running watcher: inotify descriptors %d, want %d", scannertest.InotifyInstances(t), before+1)
	}

	fixture.watcher.Sync(false)
	if running(fixture.watcher) || scannertest.InotifyInstances(t) != before {
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

func TestWatcherWaitsForAWriterThatChangedTheFileAttributes(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	path := scannertest.WriteFile(t, fixture.path("Movie (2001)", "movie.mkv"), "start")
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("expected the catch-up first")
	}
	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handle.WriteString(" more")
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chmod(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	fixture.scanner.quiet(t, sessionTestTiming.writeIdle/2)
	err = handle.Close()
	if err != nil {
		t.Fatal(err)
	}
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Movie (2001)/movie.mkv"}) {
		t.Fatalf("after the close: %q", got)
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
	fixture.watcher.timing.maxBatch = 50 * time.Millisecond
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
	err = os.Rename(fixture.path("New (2020)", "new.mkv"), fixture.path("New (2020)", "renamed.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/new.mkv", "New (2020)/renamed.mkv"}) {
		t.Fatalf("polled rename split across runs: %q", got)
	}
	temp := scannertest.WriteFile(t, fixture.path("New (2020)", "download.part"), "replacement")
	err = os.Rename(temp, fixture.path("New (2020)", "renamed.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	got = sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/renamed.mkv"}) {
		t.Fatalf("polled atomic replacement: %q", got)
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

// lossyNotifier is the real inotify instance with a switch that drops its
// events, as a full kernel queue does, so a test can stage what an overflow
// hides.
type lossyNotifier struct {
	notifier
	out    chan rawEvent
	losing atomic.Bool
	lost   chan rawEvent
}

func newLossyNotifier(t *testing.T) (*lossyNotifier, func(*sync.WaitGroup) (notifier, error)) {
	t.Helper()
	l := &lossyNotifier{out: make(chan rawEvent, 4096), lost: make(chan rawEvent, 4096)}
	open := func(wait *sync.WaitGroup) (notifier, error) {
		inner, err := newNotifier(wait)
		if err != nil {
			return nil, err
		}
		l.notifier = inner
		wait.Go(func() {
			defer close(l.out)
			for event := range inner.events() {
				if l.losing.Load() {
					l.lost <- event
					continue
				}
				l.out <- event
			}
		})
		return l, nil
	}
	return l, open
}

func (l *lossyNotifier) events() <-chan rawEvent { return l.out }

// lose drops events until one with want arrives.
func (l *lossyNotifier) lose(t *testing.T, want op) {
	t.Helper()
	for {
		select {
		case event := <-l.lost:
			if event.op&want != 0 {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the expected event never arrived")
		}
	}
}

func TestWatcherFollowsDirectoriesMovedWhileEventsWereLost(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scannertest.WriteFile(t, fixture.path("Old (2001)", "old.mkv"), "old")
	scannertest.WriteFile(t, fixture.path("Gone (2002)", "gone.mkv"), "gone")
	lossy, open := newLossyNotifier(t)
	fixture.watcher.open = open
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("expected the first catch-up")
	}

	// A directory renamed inside the library keeps its inode, and with it its
	// watch descriptor; another moves out of the library.
	lossy.losing.Store(true)
	err := os.Rename(fixture.path("Old (2001)"), fixture.path("Renamed (2001)"))
	if err != nil {
		t.Fatal(err)
	}
	lossy.lose(t, opMovedTo)
	outside := t.TempDir()
	err = os.Rename(fixture.path("Gone (2002)"), filepath.Join(outside, "Gone (2002)"))
	if err != nil {
		t.Fatal(err)
	}
	lossy.lose(t, opMovedFrom)
	lossy.losing.Store(false)
	lossy.out <- rawEvent{wd: -1, op: opOverflow}
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a queue overflow must be followed by a catch-up")
	}

	scannertest.WriteFile(t, filepath.Join(outside, "Gone (2002)", "elsewhere.mkv"), "elsewhere")
	scannertest.WriteFile(t, fixture.path("Renamed (2001)", "new.mkv"), "new")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"Renamed (2001)/new.mkv"}) {
		t.Fatalf("after the overflow: %q", got)
	}
	fixture.scanner.quiet(t, 300*time.Millisecond)
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

func TestWatcherPollsWhenTheNotifierStops(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scripted := newScriptedNotifier()
	fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return scripted, nil }
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("expected the first catch-up")
	}
	close(scripted.out)
	// Moving to the poller arms the library again, which walks it once.
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a library moved to the poller must be walked once")
	}
	scannertest.WriteFile(t, fixture.path("New (2020)", "new.mkv"), "new")
	got := sortedPaths(fixture.scanner.nextChanges(t))
	if !slices.Equal(got, []string{"New (2020)/new.mkv"}) {
		t.Fatalf("polled create after the notifier stopped: %q", got)
	}
}

func TestWatcherPausesOnUnmountUntilTheLibraryIsBack(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	scripted := newScriptedNotifier()
	fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return scripted, nil }
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("expected the first catch-up")
	}
	// The root is the first directory watched.
	scripted.out <- rawEvent{wd: 1, op: opUnmount}
	// The root is still present, so the next layout check arms it again and
	// walks it once rather than trusting what it missed.
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a library back after an unmount must be walked once")
	}
}

func TestWatcherIdlesALibraryWhoseDirectoryIsCleared(t *testing.T) {
	var mu sync.Mutex
	root := t.TempDir()
	configured := true
	fixture := newWatcherFixture(t, func(lib *Library) {
		lib.Directory = func() sql.NullString {
			mu.Lock()
			defer mu.Unlock()
			return sql.NullString{String: root, Valid: configured}
		}
	})
	fixture.watcher.Sync(true)
	fixture.scanner.next(t)

	mu.Lock()
	configured = false
	mu.Unlock()
	fixture.watcher.Sync(true)
	scannertest.WriteFile(t, filepath.Join(root, "Ignored (2020)", "ignored.mkv"), "x")
	fixture.scanner.quiet(t, 400*time.Millisecond)

	mu.Lock()
	configured = true
	mu.Unlock()
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a library configured again must be walked once")
	}
}

func TestWatcherPausesALibraryWhoseRootDoesNotAnswer(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	probes := make(chan struct{}, 10)
	fixture.watcher.probe = func(root string) rootProbe {
		probes <- struct{}{}
		<-hang
		return probeRoot(root)
	}
	fixture.watcher.Sync(true)
	scannertest.WaitForSignal(t, probes, 5*time.Second, "the root probe")
	// Nothing is dispatched for a library that does not answer, and the
	// hanging stat is not retried while it still hangs.
	fixture.scanner.quiet(t, 1500*time.Millisecond)
	if len(probes) != 0 {
		t.Fatalf("%d more probes started while the first still hung", len(probes))
	}
	started := time.Now()
	fixture.watcher.Sync(false)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("turning the watcher off waited %s on a root that does not answer", elapsed)
	}
}

func TestWatcherStartsNothingOnceTheScanContextEnded(t *testing.T) {
	before := scannertest.InotifyInstances(t)
	fixture := newWatcherFixture(t, nil)
	fixture.cancel()
	fixture.watcher.Sync(true)
	if running(fixture.watcher) || scannertest.InotifyInstances(t) != before {
		t.Fatal("a watcher whose scan context ended must not start a session")
	}
	fixture.scanner.quiet(t, 300*time.Millisecond)
}

// TestWatcherAppliesNothingDuringALibraryScan drives the scanners' real
// launcher: the watcher keeps collecting events while a library scan runs,
// but every run it asks for is refused until the scan ends, and what it saw
// meanwhile is applied afterwards.
func TestWatcherAppliesNothingDuringALibraryScan(t *testing.T) {
	var scans sync.WaitGroup
	launcher := &scanner.Launcher{Wait: &scans}
	var scanning, overlapped atomic.Bool
	runs := make(chan scanner.Changes, 100)
	requested := make(chan struct{}, 100)
	fixture := newWatcherFixture(t, func(lib *Library) {
		lib.StartChanges = func(changes scanner.Changes) scanner.StartResult {
			requested <- struct{}{}
			return launcher.Start(lib.Directory(), &changes, func(scanner.ScanScope) {}, func(_ string, changes *scanner.Changes) {
				if scanning.Load() {
					overlapped.Store(true)
				}
				runs <- *changes
			})
		}
	})
	t.Cleanup(scans.Wait)
	release := make(chan struct{})
	scanning.Store(true)
	launcher.Start(sql.NullString{String: fixture.root, Valid: true}, nil, func(scanner.ScanScope) {}, func(string, *scanner.Changes) {
		<-release
		scanning.Store(false)
	})

	fixture.watcher.Sync(true)
	// The catch-up is asked for once the library is armed; it is refused.
	scannertest.WaitForSignal(t, requested, 5*time.Second, "the catch-up request")
	scannertest.WriteFile(t, fixture.path("New (2020)", "new.mkv"), "new")
	select {
	case changes := <-runs:
		t.Fatalf("a run started during the library scan: %+v", changes)
	case <-time.After(time.Second):
	}

	close(release)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case changes := <-runs:
			if overlapped.Load() {
				t.Fatal("a watcher run overlapped the library scan")
			}
			if slices.Equal(changes.Paths, []string{"New (2020)/new.mkv"}) {
				return
			}
		case <-deadline:
			t.Fatal("the change seen during the scan was never applied")
		}
	}
}

func TestWatcherSettlesPolledDeferralsAgain(t *testing.T) {
	fixture := newWatcherFixture(t, nil)
	fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return nil, errNotifierUnsupported }
	fixture.watcher.Sync(true)
	if !fixture.scanner.next(t).CatchUp {
		t.Fatal("a polled library catches up once its snapshot is taken")
	}
	path := scannertest.WriteFile(t, fixture.path("New (2020)", "new.mkv"), "new")
	changes := fixture.scanner.nextChanges(t)
	if !slices.Equal(changes.Paths, []string{"New (2020)/new.mkv"}) {
		t.Fatalf("polled create: %q", changes.Paths)
	}
	// The scanner found the file changing again and deferred it; the
	// poller must see it hold still through its own quiet period before
	// the batch's shorter one may dispatch it.
	err := os.WriteFile(path, []byte("new, resumed"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	changes.OnDeferred("New (2020)/new.mkv", time.Time{})
	fixture.scanner.quiet(t, sessionTestPolling.quiet*3/4)
	again := fixture.scanner.nextChanges(t)
	if !slices.Equal(again.Paths, []string{"New (2020)/new.mkv"}) {
		t.Fatalf("deferred polled file: %q", again.Paths)
	}
}

func TestProbeRootResolvesARelativeRoot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	err := os.Mkdir(filepath.Join(dir, "lib"), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(dir, "lib"))
	if err != nil {
		t.Fatal(err)
	}
	probe := probeRoot("lib")
	if probe.err != nil || probe.resolved != want {
		t.Fatalf("probeRoot(lib) = %+v, want %s", probe, want)
	}
}

func TestWatcherFollowsRetargetedParentSymlink(t *testing.T) {
	for _, polling := range []bool{false, true} {
		for _, broken := range []bool{false, true} {
			t.Run(fmt.Sprintf("polling=%v/broken=%v", polling, broken), func(t *testing.T) {
				oldParent, newParent := t.TempDir(), t.TempDir()
				oldRoot, newRoot := filepath.Join(oldParent, "media"), filepath.Join(newParent, "media")
				scannertest.WriteFile(t, filepath.Join(oldRoot, "old.mkv"), "old")
				scannertest.WriteFile(t, filepath.Join(newRoot, "Existing", "new.mkv"), "new")
				link := filepath.Join(t.TempDir(), "library")
				retargetSymlink(t, link, oldParent)
				fixture := newWatcherFixture(t, func(lib *Library) {
					lib.Directory = func() sql.NullString {
						return sql.NullString{String: filepath.Join(link, "media"), Valid: true}
					}
				})
				if polling {
					fixture.watcher.open = func(*sync.WaitGroup) (notifier, error) { return nil, errNotifierUnsupported }
				}
				// Pending old-target changes must wait longer than a layout check.
				fixture.watcher.timing.settle = time.Second
				fixture.watcher.Sync(true)
				catchUp := fixture.scanner.next(t)
				if !catchUp.CatchUp {
					t.Fatal("expected initial catch-up")
				}
				fixture.scanner.quiet(t, 3*sessionTestPolling.interval)
				scannertest.WriteFile(t, filepath.Join(oldRoot, "pending.mkv"), "obsolete")
				if broken {
					retargetSymlink(t, link, filepath.Join(t.TempDir(), "missing"))
					fixture.scanner.quiet(t, 5*sessionTestPolling.recheck)
				}
				retargetSymlink(t, link, newParent)
				catchUp = fixture.scanner.next(t)
				if !catchUp.CatchUp {
					t.Fatal("retargeting must catch up before dispatching changes")
				}
				scannertest.WriteFile(t, filepath.Join(oldRoot, "obsolete.mkv"), "obsolete")
				scannertest.WriteFile(t, filepath.Join(newRoot, "fresh.mkv"), "fresh")
				got := fixture.scanner.next(t)
				if got.CatchUp || !slices.Equal(got.Paths, []string{"fresh.mkv"}) {
					t.Fatalf("after retargeting: %+v", got)
				}
				fixture.scanner.quiet(t, 3*sessionTestPolling.interval)
			})
		}
	}
}

func TestSessionRearmsRetargetedPollBeforeApplyingChanges(t *testing.T) {
	root := t.TempDir()
	stopped := false
	lib := &library{
		Library: Library{Directory: func() sql.NullString { return sql.NullString{String: root, Valid: true} }},
		root:    root, resolved: t.TempDir(), mode: modePoll, generation: 2,
		pending: newPending(defaultTiming), stopPoll: func() { stopped = true },
	}
	lib.pending.note("obsolete.mkv", changeRemoved, time.Now(), false)
	s := &session{
		ctx: context.Background(), deps: Dependencies{Now: time.Now, Logger: &scannertest.Logger{}},
		timing: defaultTiming, polling: defaultPollTiming, probe: probeRoot,
		notifier: newScriptedNotifier(), watches: make(map[int32]*watchedDir), dirs: make(map[string]int32),
	}
	s.handlePoll(pollReport{
		library: lib, generation: 2, retargeted: true,
		changes: []polledChange{{rel: "also-obsolete.mkv", kind: changeRemoved}},
	})
	if !stopped || lib.mode != modeInotify || lib.resolved != root || !lib.catchUp || !lib.pending.empty() {
		t.Fatalf("retarget did not replace the poller and discard old changes: %+v", lib)
	}
	// Even a matching generation is stale after switching to inotify.
	s.handlePoll(pollReport{library: lib, generation: 2, offline: true})
	if lib.offline {
		t.Fatal("old poller paused the new inotify backend")
	}
}

func TestLayoutCheckPausesWhileAnEarlierProbeStillHangs(t *testing.T) {
	root := t.TempDir()
	lib := &library{root: root, resolved: root, mode: modeInotify, catchUp: true, pending: newPending(defaultTiming)}
	lib.pending.note("pending.mkv", changeRemoved, time.Now(), false)
	lib.probing.Store(true)
	s := &session{
		ctx: context.Background(), deps: Dependencies{Now: time.Now, Logger: &scannertest.Logger{}},
		timing: defaultTiming, polling: defaultPollTiming,
	}
	s.checkLayout(lib)
	if !lib.offline || lib.catchUp || !lib.pending.empty() {
		t.Fatal("a check that could not resolve the root left work eligible for dispatch")
	}
}
