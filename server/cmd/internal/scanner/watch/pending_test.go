package watch

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

var testTiming = timing{
	settle:    2 * time.Second,
	maxBatch:  30 * time.Second,
	writeIdle: 60 * time.Second,
	quiet:     5 * time.Second,
	retry:     5 * time.Second,
}

// fakeFiles answers inspect from a table of change times; a missing entry is
// a missing file.
type fakeFiles map[string]time.Time

func (f fakeFiles) inspect(rel string) (time.Time, bool) {
	at, ok := f[rel]
	return at, ok
}

func rels(items []batchItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.rel)
	}
	return out
}

func TestPendingBatchesABurstOnceTheLibraryIsQuiet(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{}
	p := newPending(testTiming)
	for i, rel := range []string{"Album/01.flac", "Album/02.flac", "Album/03.flac"} {
		at := start.Add(time.Duration(i) * time.Second)
		files[rel] = at.Add(-10 * time.Second)
		p.note(rel, changeSettled, at, false)
	}
	if due := p.due(); !due.Equal(start.Add(4 * time.Second)) {
		t.Fatalf("due = %v, want two seconds after the last event", due)
	}
	if got := p.take(start.Add(3*time.Second), files.inspect); got != nil {
		t.Fatalf("took %q before the library was quiet", rels(got))
	}
	got := rels(p.take(start.Add(4*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"Album/01.flac", "Album/02.flac", "Album/03.flac"}) {
		t.Fatalf("batch = %q", got)
	}
	if !p.empty() || !p.due().IsZero() {
		t.Fatal("a dispatched batch must leave nothing pending")
	}
}

func TestPendingCapsTheWaitDuringAContinuousCopy(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{"done.mkv": start.Add(-time.Minute)}
	p := newPending(testTiming)
	p.note("done.mkv", changeSettled, start, false)
	// Another file keeps being written every second.
	var now time.Time
	for i := 1; i <= 31; i++ {
		now = start.Add(time.Duration(i) * time.Second)
		p.note("copying.mkv", changeWritten, now, false)
		files["copying.mkv"] = now
	}
	if due := p.due(); !due.Equal(start.Add(30 * time.Second)) {
		t.Fatalf("due = %v, want the 30s cap", due)
	}
	got := rels(p.take(start.Add(30*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"done.mkv"}) {
		t.Fatalf("batch = %q, want only the finished file", got)
	}
	// The writer is released once it closes the file and it has been quiet.
	closed := now.Add(time.Second)
	p.note("copying.mkv", changeSettled, closed, false)
	files["copying.mkv"] = closed
	if got := p.take(closed.Add(2*time.Second), files.inspect); got != nil {
		t.Fatalf("took %q inside the quiet period", rels(got))
	}
	if due := p.due(); !due.Equal(closed.Add(5 * time.Second)) {
		t.Fatalf("due = %v, want the quiet period after the close", due)
	}
	got = rels(p.take(closed.Add(5*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"copying.mkv"}) {
		t.Fatalf("batch = %q", got)
	}
}

func TestPendingReleasesAStalledWriter(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{"stalled.mkv": start}
	p := newPending(testTiming)
	p.note("stalled.mkv", changeWritten, start, false)
	if got := p.take(start.Add(2*time.Second), files.inspect); got != nil {
		t.Fatalf("took a file still being written: %q", rels(got))
	}
	if due := p.due(); !due.Equal(start.Add(60 * time.Second)) {
		t.Fatalf("due = %v, want the write idle timeout", due)
	}
	got := rels(p.take(start.Add(60*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"stalled.mkv"}) {
		t.Fatalf("batch = %q", got)
	}
}

func TestPendingKeepsAWriterOpenThroughAnAttributeChange(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{"paused.mkv": start}
	p := newPending(testTiming)
	p.note("paused.mkv", changeWritten, start, false)
	// A transfer that pauses and sets the file's timestamps or permissions
	// has still not closed it.
	p.note("paused.mkv", changeAttributes, start.Add(time.Second), false)
	if got := p.take(start.Add(10*time.Second), files.inspect); got != nil {
		t.Fatalf("took a file still open for writing: %q", rels(got))
	}
	closedAt := start.Add(20 * time.Second)
	files["paused.mkv"] = closedAt
	p.note("paused.mkv", changeSettled, closedAt, false)
	got := rels(p.take(closedAt.Add(testTiming.quiet), files.inspect))
	if !reflect.DeepEqual(got, []string{"paused.mkv"}) {
		t.Fatalf("batch after the close = %q", got)
	}
}

func TestPendingHoldsARemovalUntilTheRenamedFileSettles(t *testing.T) {
	start := time.Unix(1000, 0)
	// rename(2) updates ctime, so the new name is fresh while the old one is
	// simply gone.
	files := fakeFiles{"Show/Season 1/S01E01 - Pilot.mkv": start}
	p := newPending(testTiming)
	p.note("Show/Season 1/S01E01.mkv", changeRemoved, start, false)
	p.note("Show/Season 1/S01E01 - Pilot.mkv", changeSettled, start, false)
	if got := p.take(start.Add(2*time.Second), files.inspect); got != nil {
		t.Fatalf("dispatched %q before the new name settled", rels(got))
	}
	got := rels(p.take(start.Add(5*time.Second), files.inspect))
	want := []string{"Show/Season 1/S01E01 - Pilot.mkv", "Show/Season 1/S01E01.mkv"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batch = %q, want both names together", got)
	}

	// A removal alone goes as soon as the library is quiet.
	p.note("gone.mkv", changeRemoved, start.Add(time.Minute), false)
	got = rels(p.take(start.Add(time.Minute+2*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"gone.mkv"}) {
		t.Fatalf("batch = %q", got)
	}
}

func TestPendingReleasesAHeldRemovalAtTheCap(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{"writing.mkv": start}
	p := newPending(testTiming)
	p.note("gone.mkv", changeRemoved, start, false)
	p.note("writing.mkv", changeWritten, start, false)
	if got := p.take(start.Add(2*time.Second), files.inspect); got != nil {
		t.Fatalf("batch = %q", rels(got))
	}
	if due := p.due(); !due.Equal(start.Add(30 * time.Second)) {
		t.Fatalf("due = %v, want the removal released at the cap", due)
	}
	got := rels(p.take(start.Add(30*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"gone.mkv"}) {
		t.Fatalf("batch = %q", got)
	}
}

func TestPendingTreatsAVanishedFileAsARemovalAndTrustsVerifiedFiles(t *testing.T) {
	start := time.Unix(1000, 0)
	p := newPending(testTiming)
	p.note("vanished.mkv", changeSettled, start, false)
	// The poller already waited out the quiet period; no stat is needed.
	p.note("polled.mkv", changeSettled, start, true)
	inspected := 0
	got := p.take(start.Add(2*time.Second), func(rel string) (time.Time, bool) {
		inspected++
		return time.Time{}, false
	})
	if !reflect.DeepEqual(rels(got), []string{"polled.mkv", "vanished.mkv"}) || inspected != 1 {
		t.Fatalf("batch = %q, inspected %d", rels(got), inspected)
	}
	if !got[1].entry.removal {
		t.Fatal("a vanished file must dispatch as a removal")
	}
}

func TestPendingHoldAndDeferral(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{"a.mkv": start.Add(-time.Minute)}
	p := newPending(testTiming)
	p.note("a.mkv", changeSettled, start, false)
	batch := p.take(start.Add(2*time.Second), files.inspect)
	if len(batch) != 1 {
		t.Fatalf("batch = %q", rels(batch))
	}
	// The scanner was busy: the batch comes back after the retry delay.
	retryAt := start.Add(7 * time.Second)
	p.hold(batch[0].rel, batch[0].entry, retryAt)
	if due := p.due(); !due.Equal(retryAt) {
		t.Fatalf("due = %v, want %v", due, retryAt)
	}
	if got := p.take(start.Add(6*time.Second), files.inspect); got != nil {
		t.Fatalf("took %q before the retry", rels(got))
	}
	if got := rels(p.take(retryAt, files.inspect)); !reflect.DeepEqual(got, []string{"a.mkv"}) {
		t.Fatalf("retry batch = %q", got)
	}

	// The scanner deferred a file with a known eligibility time.
	eligible := start.Add(20 * time.Second)
	p.deferred("a.mkv", eligible, start.Add(10*time.Second))
	if due := p.due(); !due.Equal(eligible) {
		t.Fatalf("due = %v, want %v", due, eligible)
	}
	if got := rels(p.take(eligible, files.inspect)); !reflect.DeepEqual(got, []string{"a.mkv"}) {
		t.Fatalf("deferred batch = %q", got)
	}

	// Without one, it waits the quiet period and the stat decides.
	p.deferred("a.mkv", time.Time{}, start.Add(time.Minute))
	if due := p.due(); !due.Equal(start.Add(time.Minute + 5*time.Second)) {
		t.Fatalf("due = %v", due)
	}
}

func TestPendingDispatchesFilesCopiedAMomentApartTogether(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{}
	p := newPending(testTiming)
	for i, rel := range []string{"Album/01.flac", "Album/02.flac", "Album/03.flac"} {
		at := start.Add(time.Duration(i) * time.Second)
		files[rel] = at
		p.note(rel, changeSettled, at, false)
	}
	// 01 is ready at 5s, 03 only at 7s: 01 waits for the others.
	if got := p.take(start.Add(5*time.Second), files.inspect); got != nil {
		t.Fatalf("dispatched %q while the rest of the album was settling", rels(got))
	}
	if due := p.due(); !due.Equal(start.Add(7 * time.Second)) {
		t.Fatalf("due = %v, want when the last track is quiet", due)
	}
	got := rels(p.take(start.Add(7*time.Second), files.inspect))
	if !reflect.DeepEqual(got, []string{"Album/01.flac", "Album/02.flac", "Album/03.flac"}) {
		t.Fatalf("batch = %q, want the whole album", got)
	}
}

func TestPendingHoldForSettlingFilesEndsAtTheCap(t *testing.T) {
	start := time.Unix(1000, 0)
	files := fakeFiles{"first.mkv": start}
	p := newPending(testTiming)
	p.note("first.mkv", changeSettled, start, false)
	// A new file lands every four seconds, each still quiet when the
	// previous one is ready.
	var dispatched []string
	for i := 1; i <= 12 && len(dispatched) == 0; i++ {
		now := start.Add(time.Duration(i) * 4 * time.Second)
		rel := "next-" + string(rune('a'+i)) + ".mkv"
		files[rel] = now
		p.note(rel, changeSettled, now, false)
		for check := now; check.Before(now.Add(4 * time.Second)); check = check.Add(time.Second) {
			dispatched = append(dispatched, rels(p.take(check, files.inspect))...)
		}
	}
	if !slices.Contains(dispatched, "first.mkv") {
		t.Fatalf("a steady trickle starved the first file: dispatched %q", dispatched)
	}
}
