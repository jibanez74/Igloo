package watch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

const (
	// pollInterval spaces the directory stats of a polled library. Each poll
	// stats directories, inspecting files only when their directory changed
	// or they are still settling, so a NAS can spin down between polls.
	pollInterval = 3 * time.Minute
	// pollSettleInterval re-checks new files sooner while they settle.
	pollSettleInterval = 30 * time.Second
	// recheckInterval spaces the checks for a library that went offline.
	recheckInterval = 30 * time.Second
	// pollHoldCap bounds how long held removals wait for files still
	// settling: far longer than a rename's new name takes, so a file that
	// never settles cannot hold back the library's deletions for good.
	pollHoldCap = 10 * time.Minute
)

type polledDir struct {
	id      identity
	mtime   int64
	entries map[string]bool // name -> is a directory
	// unread marks a directory whose listing failed; every poll lists it
	// again until one succeeds.
	unread bool
}

type polledChange struct {
	rel  string
	kind changeKind
}

// pollReport carries what one poll found back to the session.
type pollReport struct {
	library    *library
	generation int
	changes    []polledChange
	offline    bool
	online     bool
	retargeted bool
}

// poller watches one library by polling, for filesystems inotify cannot see
// into: network shares and FUSE, and every library on macOS. It snapshots
// each directory's modification time and entry names; a directory whose
// mtime moved is listed again and diffed. It owns every filesystem access for
// its library, so a share that hangs stalls only this goroutine. New files
// are reported once they have been unchanged for the full FileQuietPeriod,
// since a poll cannot see a writer close them.
//
// Limitation: rewriting a file in place changes no directory, so it is only
// picked up by a library scan.
type poller struct {
	library        *library
	generation     int
	root           string
	configuredRoot string
	resolve        func(root string) (string, error)
	allow          func(root, path string) bool
	extensions     map[string]bool
	timing         pollTiming
	now            func() time.Time

	layout   layout
	dirs     map[string]*polledDir
	settling map[string]scanner.FileFingerprint
	held     map[string]changeKind // absolute paths ready for the next report
	// heldSince is when a held removal first waited for settling files.
	heldSince time.Time
	offline   bool
	// unresolved is set after one failed root resolution while online: a
	// symlink caught mid-swap or a parent briefly unreadable fails once, and
	// going offline for it would re-list the whole share and walk it again.
	unresolved bool

	// resettling holds the files the scanner deferred, handed back by the
	// session; they are observed again from scratch. The session never waits
	// on the poller, which may be stuck on a share that hangs.
	resettleMu     sync.Mutex
	resettling     []string
	resettleSignal chan struct{}
}

// resettle hands a deferred file back to the poller: the scanner saw it still
// changing after the poller had settled it, so it is settled again under the
// polling rules before it dispatches. It never blocks.
func (p *poller) resettle(rel string) {
	p.resettleMu.Lock()
	p.resettling = append(p.resettling, filepath.Join(p.root, rel))
	p.resettleMu.Unlock()
	select {
	case p.resettleSignal <- struct{}{}:
	default:
	}
}

// drainResettling observes the handed-back files again. While offline they are
// dropped: the catch-up walk after re-arming covers them.
func (p *poller) drainResettling() {
	p.resettleMu.Lock()
	paths := p.resettling
	p.resettling = nil
	p.resettleMu.Unlock()
	if p.offline {
		return
	}
	for _, path := range paths {
		p.observe(path)
	}
}

type pollTiming struct {
	interval time.Duration
	settle   time.Duration
	recheck  time.Duration
	quiet    time.Duration
	holdCap  time.Duration
}

var defaultPollTiming = pollTiming{interval: pollInterval, settle: pollSettleInterval, recheck: recheckInterval, quiet: scanner.FileQuietPeriod, holdCap: pollHoldCap}

func (p *poller) run(ctx context.Context, out chan<- pollReport) {
	p.offline = true
	delay := time.Duration(0)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.resettleSignal:
			p.drainResettling()
			// Record the first observation at the settling cadence rather
			// than at the next full poll.
			timer.Reset(p.nextDelay())
			continue
		case <-timer.C:
		}
		report := p.poll()
		hasNews := report.offline || report.online || report.retargeted || len(report.changes) > 0
		if hasNews {
			report.library, report.generation = p.library, p.generation
			select {
			case out <- report:
			case <-ctx.Done():
				return
			}
		}
		if report.retargeted {
			return
		}
		timer.Reset(p.nextDelay())
	}
}

func (p *poller) nextDelay() time.Duration {
	if p.offline {
		return p.timing.recheck
	}
	if p.unresolved || len(p.settling) > 0 {
		return min(p.timing.interval, p.timing.settle)
	}
	return p.timing.interval
}

func (p *poller) poll() pollReport {
	resolved, err := p.resolve(p.configuredRoot)
	if err != nil {
		if p.offline {
			return pollReport{}
		}
		if !p.unresolved {
			p.unresolved = true
			return pollReport{}
		}
		return p.goOffline()
	}
	p.unresolved = false
	if resolved != p.root {
		return pollReport{retargeted: true}
	}
	if p.offline {
		return p.arm()
	}
	present, changed := p.layout.check()
	if !present {
		return p.goOffline()
	}
	if changed {
		p.offline = true
		return p.arm()
	}
	var report pollReport
	paths := make([]string, 0, len(p.dirs))
	for path := range p.dirs {
		paths = append(paths, path)
	}
	slices.Sort(paths) // Forget removed parents before visiting their children.
	for _, path := range paths {
		lost := p.diff(path)
		if lost {
			return p.goOffline()
		}
	}
	lost := p.settle()
	if lost {
		return p.goOffline()
	}
	// The session cannot see files still settling here. Its batching cap is
	// shorter than our quiet period, so keep both sides of a rename here until
	// they can be dispatched together, even across several polls, within
	// holdCap.
	if p.holdsForSettling() {
		return report
	}
	p.heldSince = time.Time{}
	for path, kind := range p.held {
		rel, ok := relative(p.root, path)
		if ok {
			report.changes = append(report.changes, polledChange{rel: rel, kind: kind})
		}
	}
	clear(p.held)
	return report
}

// holdsForSettling reports whether held changes wait for files still
// settling: while a removal is held, until holdCap after it first waited.
func (p *poller) holdsForSettling() bool {
	if len(p.settling) == 0 {
		return false
	}
	removal := false
	for _, kind := range p.held {
		removal = removal || kind == changeRemoved
	}
	if !removal {
		return false
	}
	now := p.now()
	if p.heldSince.IsZero() {
		p.heldSince = now
	}
	return now.Before(p.heldSince.Add(p.timing.holdCap))
}

// arm takes the snapshot once the library is present; the session then
// catches up with a walk of the whole library.
func (p *poller) arm() pollReport {
	present, _ := p.layout.check()
	if !present {
		return pollReport{}
	}
	recorded, err := recordRoot(p.root)
	if err != nil {
		return pollReport{}
	}
	p.layout = recorded
	p.dirs = make(map[string]*polledDir)
	p.settling = make(map[string]scanner.FileFingerprint)
	p.held = make(map[string]changeKind)
	p.heldSince = time.Time{}
	err = p.snapshot(p.root, nil)
	if err != nil {
		return pollReport{}
	}
	p.offline = false
	return pollReport{online: true}
}

func (p *poller) goOffline() pollReport {
	p.offline, p.unresolved = true, false
	p.dirs, p.settling, p.held = nil, nil, nil
	p.heldSince = time.Time{}
	return pollReport{offline: true}
}

// snapshot records directory and everything beneath it that the scanner
// would walk; with files set it also collects the media files found. A
// directory that cannot be listed is recorded as unread, so later polls retry
// it rather than taking its empty snapshot as complete.
func (p *poller) snapshot(directory string, files *[]string) error {
	return walkLibraryDirs(p.root, directory, p.allow, p.extensions, files, func(path string, info os.FileInfo, mount bool) error {
		if mount {
			p.layout.mounts[path] = true
		}
		known := &polledDir{id: identityOf(info), mtime: info.ModTime().UnixNano(), entries: map[string]bool{}}
		listing, err := os.ReadDir(path)
		if err != nil {
			known.unread = true
		} else {
			known.entries = p.entries(path, listing)
		}
		p.dirs[path] = known
		return nil
	})
}

func (p *poller) entries(directory string, listing []fs.DirEntry) map[string]bool {
	entries := make(map[string]bool, len(listing))
	for _, entry := range listing {
		path := filepath.Join(directory, entry.Name())
		if entry.IsDir() {
			allowed := p.allow == nil || p.allow(p.root, path)
			if allowed {
				entries[entry.Name()] = true
			}
			continue
		}
		if p.extensions[helpers.FileExtension(path)] {
			entries[entry.Name()] = false
		}
	}
	return entries
}

// diff lists a directory again when its mtime or identity moved, or while it
// is unread. Removals
// are only reported from a successful listing of the same directory: a
// replaced directory, such as an empty mount point, proves nothing. It
// reports whether the share itself became unavailable.
func (p *poller) diff(directory string) bool {
	known, ok := p.dirs[directory]
	if !ok {
		return false
	}
	info, err := os.Stat(directory)
	if err != nil {
		return unavailable(err)
	}
	id := identityOf(info)
	mtime := info.ModTime().UnixNano()
	unchanged := id == known.id && mtime == known.mtime && !known.unread
	if unchanged {
		return false
	}
	if id != known.id {
		// An old pending removal is no longer evidence after a directory
		// replacement either; discard its whole snapshot before observing it.
		p.forget(directory)
		// Retain the old identity until snapshot succeeds so an unreadable
		// replacement is retried on the next poll.
		p.dirs[directory] = known
		return p.observeTree(directory)
	}
	listing, err := os.ReadDir(directory)
	if err != nil {
		return unavailable(err)
	}
	current := p.entries(directory, listing)
	for name, isDir := range known.entries {
		currentDir, exists := current[name]
		if exists && currentDir == isDir {
			continue
		}
		path := filepath.Join(directory, name)
		if isDir {
			p.forget(path)
		}
		delete(p.settling, path)
		p.held[path] = changeRemoved
	}
	for name, isDir := range current {
		wasDir, existed := known.entries[name]
		path := filepath.Join(directory, name)
		if isDir {
			if existed && wasDir {
				continue
			}
			delete(p.held, path)
			lost := p.observeTree(path)
			if lost {
				return true
			}
			continue
		}
		// An unchanged name can hide an atomic replacement. The scanner's
		// fingerprint check skips any neighboring files that did not change.
		p.observe(path)
	}
	known.id, known.mtime, known.entries, known.unread = id, mtime, current, false
	return false
}

func (p *poller) observeTree(directory string) bool {
	var files []string
	err := p.snapshot(directory, &files)
	for _, file := range files {
		p.observe(file)
	}
	return unavailable(err)
}

func (p *poller) observe(path string) {
	_, waiting := p.settling[path]
	if !waiting {
		// settle records the first observation this poll; only a subsequent
		// poll may verify it, even when the file is already old enough.
		p.settling[path] = scanner.FileFingerprint{}
	}
	delete(p.held, path)
}

func (p *poller) forget(directory string) {
	for path := range p.dirs {
		if scanner.PathWithin(directory, path) {
			delete(p.dirs, path)
		}
	}
	for path := range p.settling {
		if scanner.PathWithin(directory, path) {
			delete(p.settling, path)
		}
	}
	for path := range p.held {
		if scanner.PathWithin(directory, path) {
			delete(p.held, path)
		}
	}
}

// settle queues files whose metadata held still since the last poll and is
// past the quiet period. It reports whether the share became unavailable.
func (p *poller) settle() bool {
	now := p.now()
	for path, last := range p.settling {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			delete(p.settling, path)
			p.held[path] = changeRemoved
			continue
		}
		if err != nil {
			if unavailable(err) {
				return true
			}
			// A file that cannot be read never settles; the scanner reports it.
			delete(p.settling, path)
			p.held[path] = changeSettled
			continue
		}
		observed := scanner.FingerprintOf(info)
		stable := observed == last
		quiet := !now.Before(observed.ChangedAt().Add(p.timing.quiet))
		if !stable || !quiet {
			p.settling[path] = observed
			continue
		}
		delete(p.settling, path)
		p.held[path] = changeSettled
	}
	return false
}
