package watch

import (
	"slices"
	"time"

	"igloo/cmd/internal/scanner"
)

type changeKind uint8

const (
	// changeWritten is content still being written: IN_MODIFY.
	changeWritten changeKind = iota
	// changeSettled is a file created, closed after writing or moved in.
	changeSettled
	// changeRemoved is a file or directory deleted or moved out.
	changeRemoved
	// changeAttributes is a file given new permissions or timestamps: IN_ATTRIB.
	// It says nothing about its writer, so a file still being written stays so.
	changeAttributes
)

// timing holds the coalescer's delays; tests shorten them.
type timing struct {
	// settle is how long a library must be event-quiet before its changes
	// dispatch, so an album extraction or a season copy becomes one run.
	settle time.Duration
	// maxBatch caps that wait from the first change after a dispatch, so a
	// long copy session cannot starve the files already finished.
	maxBatch time.Duration
	// writeIdle releases a file still marked as being written once nothing
	// has touched it for this long: its writer stalled or died.
	writeIdle time.Duration
	// quiet is how long after its last change a file may dispatch.
	quiet time.Duration
	// retry is how long a batch waits when its scanner is busy: running a
	// library scan, a catch-up or another run of changes.
	retry time.Duration
	// probe is how long the session waits on a library root that does not
	// answer before pausing the library.
	probe time.Duration
}

var defaultTiming = timing{
	settle:    2 * time.Second,
	maxBatch:  30 * time.Second,
	writeIdle: scanner.FileQuietPeriod,
	quiet:     scanner.ChangeQuietPeriod,
	retry:     5 * time.Second,
	probe:     10 * time.Second,
}

type pendingPath struct {
	firstSeen time.Time
	lastEvent time.Time
	writing   bool
	removal   bool
	// verified marks a file the poller already watched through the quiet
	// period, so it dispatches without another stat.
	verified bool
	// notBefore holds a file back after a deferral or a busy scanner.
	notBefore time.Time
}

// inspectFunc stats a pending file: when it last changed and whether it
// still exists.
type inspectFunc func(rel string) (changedAt time.Time, exists bool)

// pending coalesces one library's changes, keyed by path relative to the
// library root, into batches. It is owned by the session goroutine.
type pending struct {
	timing timing
	paths  map[string]*pendingPath
	// dirty is set by an event since the last take; the batch then waits
	// for the library to be quiet, from last, capped from window.
	dirty  bool
	window time.Time
	last   time.Time
	// wakeAt is when a held path can next become ready.
	wakeAt time.Time
	// heldSince is when take first held back ready paths for others that
	// were about to settle; the hold ends maxBatch later at the latest.
	heldSince time.Time
}

func newPending(t timing) *pending {
	return &pending{timing: t, paths: make(map[string]*pendingPath)}
}

func (p *pending) empty() bool {
	return len(p.paths) == 0
}

func (p *pending) note(rel string, kind changeKind, now time.Time, verified bool) {
	entry, ok := p.paths[rel]
	if !ok {
		entry = &pendingPath{firstSeen: now}
		p.paths[rel] = entry
	}
	entry.lastEvent = now
	entry.notBefore = time.Time{}
	entry.writing = kind == changeWritten || (kind == changeAttributes && entry.writing)
	entry.removal = kind == changeRemoved
	entry.verified = kind == changeSettled && verified
	if !p.dirty {
		p.dirty, p.window = true, now
	}
	p.last = now
}

// hold puts a path back with a time it may not dispatch before: a file the
// scanner deferred, or a batch its busy scanner refused. A path that has seen
// a newer event keeps that event's state.
func (p *pending) hold(rel string, entry pendingPath, at time.Time) {
	current, ok := p.paths[rel]
	if ok {
		if current.notBefore.Before(at) && !current.lastEvent.After(entry.lastEvent) {
			current.notBefore = at
		}
	} else {
		entry.notBefore = at
		p.paths[rel] = &entry
	}
	p.wake(at)
}

// deferred re-arms a file the scanner deferred; a zero eligibleAt waits the
// quiet period and lets the stat decide.
func (p *pending) deferred(rel string, eligibleAt, now time.Time) {
	if eligibleAt.IsZero() {
		eligibleAt = now.Add(p.timing.quiet)
	}
	p.hold(rel, pendingPath{firstSeen: now, lastEvent: now}, eligibleAt)
}

func (p *pending) wake(at time.Time) {
	if p.wakeAt.IsZero() || at.Before(p.wakeAt) {
		p.wakeAt = at
	}
}

func (p *pending) batchAt() time.Time {
	quiet := p.last.Add(p.timing.settle)
	capped := p.window.Add(p.timing.maxBatch)
	return minTime(quiet, capped)
}

// due is when take should next run; zero when nothing is pending.
func (p *pending) due() time.Time {
	if p.empty() {
		return time.Time{}
	}
	if p.dirty {
		return p.batchAt()
	}
	return p.wakeAt
}

type batchItem struct {
	rel   string
	entry pendingPath
}

// take removes and returns the paths ready to dispatch, sorted. A file is
// ready once it is not being written and has been unchanged for the quiet
// period; a removal waits while another path in the library is still
// settling, within maxBatch, so both names of a rename land in one run. Ready
// paths wait for those removals, which may be the other name of one of them,
// and for files that only need to finish their quiet period, so files copied
// a moment apart, an album's tracks, dispatch as one run; the hold lasts
// maxBatch at most, so a steady trickle of new files cannot starve them.
func (p *pending) take(now time.Time, inspect inspectFunc) []batchItem {
	if p.empty() {
		return nil
	}
	if p.dirty && now.Before(p.batchAt()) {
		return nil
	}
	p.dirty, p.wakeAt = false, time.Time{}
	var ready []batchItem
	var removals []string
	settling := false
	// holdUntil is when the last path the ready ones wait for is released.
	holdUntil := time.Time{}
	for rel, entry := range p.paths {
		if entry.removal {
			removals = append(removals, rel)
			continue
		}
		at, exists := p.readyAt(rel, entry, now, inspect)
		if !exists {
			entry.removal = true
			removals = append(removals, rel)
			continue
		}
		if at.After(now) {
			settling = true
			p.wake(at)
			if !entry.writing && at.After(holdUntil) {
				holdUntil = at
			}
			continue
		}
		ready = append(ready, batchItem{rel: rel, entry: *entry})
	}
	for _, rel := range removals {
		entry := p.paths[rel]
		released := entry.firstSeen.Add(p.timing.maxBatch)
		held := settling && now.Before(released)
		if entry.notBefore.After(now) {
			released, held = entry.notBefore, true
		}
		if held {
			p.wake(released)
			if released.After(holdUntil) {
				holdUntil = released
			}
			continue
		}
		ready = append(ready, batchItem{rel: rel, entry: *entry})
	}
	if len(ready) > 0 && !holdUntil.IsZero() {
		if p.heldSince.IsZero() {
			p.heldSince = now
		}
		released := p.heldSince.Add(p.timing.maxBatch)
		if now.Before(released) {
			// Wake once the last of them is quiet rather than at each one.
			p.wakeAt = minTime(holdUntil, released)
			return nil
		}
	}
	p.heldSince = time.Time{}
	for _, item := range ready {
		delete(p.paths, item.rel)
	}
	slices.SortFunc(ready, func(a, b batchItem) int {
		if a.rel < b.rel {
			return -1
		}
		if a.rel > b.rel {
			return 1
		}
		return 0
	})
	return ready
}

func (p *pending) readyAt(rel string, entry *pendingPath, now time.Time, inspect inspectFunc) (time.Time, bool) {
	if entry.notBefore.After(now) {
		return entry.notBefore, true
	}
	stillWriting := entry.writing && now.Before(entry.lastEvent.Add(p.timing.writeIdle))
	if stillWriting {
		return entry.lastEvent.Add(p.timing.writeIdle), true
	}
	if entry.verified {
		return now, true
	}
	changedAt, exists := inspect(rel)
	if !exists {
		return time.Time{}, false
	}
	return changedAt.Add(p.timing.quiet), true
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
