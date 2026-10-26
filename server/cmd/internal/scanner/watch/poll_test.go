package watch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
)

type pollFixture struct {
	p   *poller
	now time.Time
}

func newPollFixture(t *testing.T, files ...string) *pollFixture {
	t.Helper()
	root := t.TempDir()
	for _, rel := range files {
		scannertest.WriteFile(t, filepath.Join(root, rel), "media")
	}
	return armPollFixture(t, root)
}

func armPollFixture(t *testing.T, root string) *pollFixture {
	t.Helper()
	f := &pollFixture{now: time.Now()}
	f.p = &poller{root: root, extensions: map[string]bool{"mkv": true}, timing: defaultPollTiming,
		now: func() time.Time { return f.now }, layout: layout{root: root}, offline: true}
	report := f.p.poll()
	if !report.online {
		t.Fatalf("arm: %+v", report)
	}
	return f
}

func (f *pollFixture) poll(t *testing.T, advance time.Duration, want map[string]changeKind) {
	t.Helper()
	f.now = f.now.Add(advance)
	report := f.p.poll()
	got := make(map[string]changeKind)
	for _, change := range report.changes {
		_, duplicate := got[change.rel]
		if duplicate {
			t.Fatalf("duplicate change: %+v", report.changes)
		}
		got[change.rel] = change.kind
	}
	if want == nil {
		want = map[string]changeKind{}
	}
	if report.online || report.offline || !reflect.DeepEqual(got, want) {
		t.Fatalf("poll: %+v, want changes %v", report, want)
	}
}

func TestPollRenameWaitsForReplacement(t *testing.T) {
	for _, tc := range []struct{ name, old, next, removed, added string }{
		{"file", "Show/Season 1/a.mkv", "Show/Season 1/b.mkv", "Show/Season 1/a.mkv", "Show/Season 1/b.mkv"},
		{"cross directory", "Show/Season 1/a.mkv", "Show/Season 2/b.mkv", "Show/Season 1/a.mkv", "Show/Season 2/b.mkv"},
		{"directory", "Show/Season 1", "Show/Season 2", "Show/Season 1", "Show/Season 2/a.mkv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPollFixture(t, "Show/Season 1/a.mkv")
			err := os.MkdirAll(filepath.Dir(filepath.Join(f.p.root, tc.next)), 0700)
			if err != nil {
				t.Fatal(err)
			}
			err = os.Rename(filepath.Join(f.p.root, tc.old), filepath.Join(f.p.root, tc.next))
			if err != nil {
				t.Fatal(err)
			}
			f.poll(t, 0, nil)
			// The session's 30-second cap cannot release a removal while its
			// replacement is waiting for the poller's 60-second quiet period.
			f.poll(t, 40*time.Second, nil)
			f.poll(t, 30*time.Second, map[string]changeKind{tc.removed: changeRemoved, tc.added: changeSettled})
			f.poll(t, time.Minute, nil)
		})
	}
}

func TestPollAtomicReplacementAtExistingName(t *testing.T) {
	f := newPollFixture(t, "movie.mkv")
	path := filepath.Join(f.p.root, "movie.mkv")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	temp := scannertest.WriteFile(t, filepath.Join(f.p.root, "download.part"), "other")
	err = os.Chtimes(temp, info.ModTime(), info.ModTime())
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(temp, path)
	if err != nil {
		t.Fatal(err)
	}
	f.poll(t, 0, nil)
	f.poll(t, 70*time.Second, map[string]changeKind{"movie.mkv": changeSettled})
	f.poll(t, time.Minute, nil)
}

func TestPollRequiresSeparateStableObservation(t *testing.T) {
	f := newPollFixture(t)
	scannertest.WriteFile(t, filepath.Join(f.p.root, "new.mkv"), "media")
	// An already-quiet file still needs an unchanged observation on a later poll.
	f.poll(t, 2*time.Minute, nil)
	f.poll(t, time.Second, map[string]changeKind{"new.mkv": changeSettled})
}

func TestPollRemovalLifecycle(t *testing.T) {
	for _, action := range []string{"vanish", "reappear", "rewrite", "offline", "rearm"} {
		t.Run(action, func(t *testing.T) {
			f := newPollFixture(t, "old.mkv")
			old := filepath.Join(f.p.root, "old.mkv")
			next := filepath.Join(f.p.root, "new.mkv")
			err := os.Rename(old, next)
			if err != nil {
				t.Fatal(err)
			}
			f.poll(t, 0, nil)
			switch action {
			case "vanish":
				err = os.Remove(next)
				if err != nil {
					t.Fatal(err)
				}
				f.poll(t, time.Second, map[string]changeKind{"old.mkv": changeRemoved, "new.mkv": changeRemoved})
			case "reappear":
				err = os.Rename(next, old)
				if err != nil {
					t.Fatal(err)
				}
				f.poll(t, 0, nil)
				f.poll(t, 70*time.Second, map[string]changeKind{"old.mkv": changeSettled, "new.mkv": changeRemoved})
			case "rewrite":
				scannertest.WriteFile(t, next, "still copying")
				f.poll(t, 70*time.Second, nil)
				f.poll(t, time.Second, map[string]changeKind{"old.mkv": changeRemoved, "new.mkv": changeSettled})
			case "offline", "rearm":
				if action == "offline" {
					report := f.p.goOffline()
					if !report.offline {
						t.Fatal("missing offline report")
					}
				}
				report := f.p.arm()
				if !report.online {
					t.Fatal("missing catch-up report")
				}
				f.poll(t, 70*time.Second, nil)
			}
		})
	}
}

func TestPollStandaloneDeletionAndFiltering(t *testing.T) {
	f := newPollFixture(t, "old.mkv")
	f.p.allow = func(root, path string) bool { return filepath.Base(path) != "Extras" }
	err := os.Remove(filepath.Join(f.p.root, "old.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	scannertest.WriteFile(t, filepath.Join(f.p.root, "download.part"), "temporary")
	scannertest.WriteFile(t, filepath.Join(f.p.root, "Extras", "ignored.mkv"), "extra")
	f.poll(t, 0, map[string]changeKind{"old.mkv": changeRemoved})
	f.poll(t, 70*time.Second, nil)
}

func TestPollKeepsCompletedAdditionsWithHeldRemovals(t *testing.T) {
	f := newPollFixture(t, "old.mkv")
	err := os.Rename(filepath.Join(f.p.root, "old.mkv"), filepath.Join(f.p.root, "new.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	later := scannertest.WriteFile(t, filepath.Join(f.p.root, "later.mkv"), "copying")
	future := f.now.Add(2 * time.Minute)
	err = os.Chtimes(later, future, future)
	if err != nil {
		t.Fatal(err)
	}
	f.poll(t, 0, nil)
	f.poll(t, 70*time.Second, nil)
	f.poll(t, 2*time.Minute, map[string]changeKind{
		"old.mkv": changeRemoved, "new.mkv": changeSettled, "later.mkv": changeSettled,
	})
	f.poll(t, time.Minute, nil)
}

func TestPollSessionRetriesCompleteRenameBatch(t *testing.T) {
	f := newPollFixture(t, "old.mkv")
	var calls [][]string
	lib := &library{mode: modePoll, root: f.p.root, resolved: f.p.root, pending: newPending(defaultTiming)}
	lib.StartChanges = func(changes scanner.Changes) scanner.StartResult {
		calls = append(calls, changes.Paths)
		if len(calls) == 1 {
			return scanner.StartResult{Status: scanner.StartAlreadyRunning}
		}
		return scanner.StartResult{Status: scanner.StartStarted}
	}
	s := &session{deps: Dependencies{Now: f.p.now, Logger: &scannertest.Logger{}}, timing: defaultTiming, libraries: []*library{lib}}
	poll := func(advance time.Duration) {
		f.now = f.now.Add(advance)
		report := f.p.poll()
		report.library = lib
		s.handlePoll(report)
	}
	err := os.Rename(filepath.Join(f.p.root, "old.mkv"), filepath.Join(f.p.root, "new.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	poll(0)
	f.now = f.now.Add(3 * time.Second)
	s.dispatch(f.now)
	poll(37 * time.Second)
	s.dispatch(f.now)
	if len(calls) != 0 {
		t.Fatalf("premature dispatch: %v", calls)
	}
	poll(30 * time.Second)
	f.now = f.now.Add(defaultTiming.settle)
	s.dispatch(f.now)
	f.now = f.now.Add(defaultTiming.retry)
	s.dispatch(f.now)
	want := [][]string{{"new.mkv", "old.mkv"}, {"new.mkv", "old.mkv"}}
	if !reflect.DeepEqual(calls, want) || !lib.pending.empty() {
		t.Fatalf("busy scanner dispatches = %v, pending=%v", calls, lib.pending.paths)
	}
}

func TestPollerReportsNoRemovalsFromAReplacedDirectory(t *testing.T) {
	for _, pendingRename := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "held rename"}[pendingRename], func(t *testing.T) {
			f := newPollFixture(t, "Share/a.mkv")
			if pendingRename {
				err := os.Rename(filepath.Join(f.p.root, "Share/a.mkv"), filepath.Join(f.p.root, "Share/b.mkv"))
				if err != nil {
					t.Fatal(err)
				}
				f.poll(t, 0, nil)
			}
			// A replacement directory proves nothing about missing files, just
			// like an empty mount point. Pre-create it to prevent inode reuse.
			empty := filepath.Join(t.TempDir(), "empty")
			err := os.Mkdir(empty, 0700)
			if err != nil {
				t.Fatal(err)
			}
			err = os.RemoveAll(filepath.Join(f.p.root, "Share"))
			if err != nil {
				t.Fatal(err)
			}
			err = os.Rename(empty, filepath.Join(f.p.root, "Share"))
			if err != nil {
				t.Fatal(err)
			}
			f.poll(t, 70*time.Second, nil)
			f.poll(t, time.Minute, nil)
		})
	}
}

func TestPollerPausesWhileTheRootIsGoneAndReportsNoRemovals(t *testing.T) {
	f := newPollFixture(t, "a.mkv")
	err := os.RemoveAll(f.p.root)
	if err != nil {
		t.Fatal(err)
	}
	report := f.p.poll()
	if !report.offline || len(report.changes) != 0 {
		t.Fatalf("root gone: %+v", report)
	}
	report = f.p.poll()
	if report.offline || report.online || len(report.changes) != 0 {
		t.Fatalf("still gone: %+v", report)
	}
	scannertest.WriteFile(t, filepath.Join(f.p.root, "b.mkv"), "media")
	report = f.p.poll()
	if !report.online || len(report.changes) != 0 {
		t.Fatalf("root back: %+v", report)
	}
	// The catch-up walk the online report asks for covers what changed.
	f.poll(t, 70*time.Second, nil)
}

func TestPollAnUnreadableFileDoesNotHoldRemovalsBack(t *testing.T) {
	f := newPollFixture(t, "a.mkv")
	err := os.Remove(filepath.Join(f.p.root, "a.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	// A symlink to itself never stats, so it can never settle: it goes to the
	// scanner, which reports it, and the removal with it.
	err = os.Symlink("loop.mkv", filepath.Join(f.p.root, "loop.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	f.poll(t, 0, map[string]changeKind{"a.mkv": changeRemoved, "loop.mkv": changeSettled})
	f.poll(t, time.Minute, nil)
}

func TestPollHoldForSettlingFilesEndsAtTheCap(t *testing.T) {
	f := newPollFixture(t, "a.mkv")
	f.p.timing.holdCap = time.Minute
	err := os.Remove(filepath.Join(f.p.root, "a.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	growing := scannertest.WriteFile(t, filepath.Join(f.p.root, "recording.mkv"), "r")
	f.poll(t, 0, nil)
	content := "r"
	for _, advance := range []time.Duration{30 * time.Second, 29 * time.Second} {
		content += "r"
		scannertest.WriteFile(t, growing, content)
		f.poll(t, advance, nil)
	}
	content += "r"
	scannertest.WriteFile(t, growing, content)
	// A file still being written holds the removal back for the cap, then
	// the removal goes alone.
	f.poll(t, 2*time.Second, map[string]changeKind{"a.mkv": changeRemoved})
}

func TestPollResettlesADeferredFile(t *testing.T) {
	f := newPollFixture(t, "a.mkv")
	path := filepath.Join(f.p.root, "a.mkv")
	f.p.resettleSignal = make(chan struct{}, 1)
	// The scanner saw the file change after the poller had settled it.
	err := os.WriteFile(path, []byte("still copying"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.p.resettle("a.mkv")
	f.p.drainResettling()
	if _, settling := f.p.settling[path]; !settling {
		t.Fatal("a handed-back file is observed again")
	}
	// A first observation, then the quiet period, before it is reported.
	f.poll(t, 0, nil)
	f.poll(t, 30*time.Second, nil)
	err = os.WriteFile(path, []byte("still copying, more"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	// Changed metadata starts the wait over, as for any settling file.
	f.poll(t, 70*time.Second, nil)
	f.poll(t, 70*time.Second, map[string]changeKind{"a.mkv": changeSettled})
}

// lockDirectory makes directory unreadable until the returned function, or the
// end of the test, unlocks it.
func lockDirectory(t *testing.T, directory string) func() {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	err := os.MkdirAll(directory, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	unlock := func() {
		err := os.Chmod(directory, 0o755)
		if err != nil {
			t.Fatal(err)
		}
	}
	err = os.Chmod(directory, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o755) })
	return unlock
}

func TestPollRetriesADirectoryThatCouldNotBeListed(t *testing.T) {
	t.Run("found by a poll", func(t *testing.T) {
		f := newPollFixture(t)
		unlock := lockDirectory(t, filepath.Join(f.p.root, "Sub"))
		f.poll(t, 0, nil)
		unlock()
		scannertest.WriteFile(t, filepath.Join(f.p.root, "Sub", "a.mkv"), "media")
		scannertest.WriteFile(t, filepath.Join(f.p.root, "Sub", "Deeper", "b.mkv"), "media")
		f.poll(t, 0, nil)
		f.poll(t, 70*time.Second, map[string]changeKind{"Sub/a.mkv": changeSettled, "Sub/Deeper/b.mkv": changeSettled})
		f.poll(t, time.Minute, nil)
	})
	t.Run("unreadable when armed", func(t *testing.T) {
		root := t.TempDir()
		scannertest.WriteFile(t, filepath.Join(root, "Sub", "a.mkv"), "media")
		unlock := lockDirectory(t, filepath.Join(root, "Sub"))
		f := armPollFixture(t, root)
		f.poll(t, 0, nil)
		unlock()
		// The catch-up walk could not read it either, so its files are new.
		f.poll(t, 0, nil)
		f.poll(t, 70*time.Second, map[string]changeKind{"Sub/a.mkv": changeSettled})
		f.poll(t, time.Minute, nil)
	})
}
