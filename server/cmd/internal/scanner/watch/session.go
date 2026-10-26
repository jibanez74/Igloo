package watch

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

// errNetworkMount aborts an inotify setup that walked into a mount inotify
// cannot see into; the library polls instead.
var errNetworkMount = errors.New("network or FUSE mount inside the library")

type backendMode uint8

const (
	modeIdle backendMode = iota
	modeInotify
	modePoll
)

func (m backendMode) String() string {
	switch m {
	case modeInotify:
		return "inotify"
	case modePoll:
		return "poll"
	default:
		return "idle"
	}
}

// library is the session's state for one configured library.
type library struct {
	Library
	// root is the configured directory, as the scanner walks it; resolved has
	// its symlinks resolved, and watches, events and pending paths use it.
	// A path relative to one is relative to the other.
	root     string
	resolved string
	// linked is set when the root resolves through a symlink, which can be
	// retargeted between two layout checks.
	linked  bool
	mode    backendMode
	layout  layout
	pending *pending
	// offline pauses a library whose root or a mount inside it went away;
	// checkAt is the next look at its layout, offline or not.
	offline bool
	checkAt time.Time
	// catchUp asks for a walk of the whole library, after the watcher could
	// not see what changed: when it arms, after an overflow, after an outage.
	catchUp bool
	retryAt time.Time
	// generation tells a stopped poller's late reports apart.
	generation int
	poller     *poller
	stopPoll   context.CancelFunc
	// probing is set while a bounded check of the root is still running.
	probing atomic.Bool
}

// rootProbe is what arming learns about a library root: its path with
// symlinks resolved, and the network or FUSE filesystem holding it or mounted
// inside it, if any.
type rootProbe struct {
	resolved string
	fsName   string
	err      error
}

func probeRoot(resolve func(string) (string, error), root string) rootProbe {
	resolved, err := resolve(root)
	if err != nil {
		return rootProbe{err: err}
	}
	fsName, err := networkFilesystem(resolved)
	return rootProbe{resolved: resolved, fsName: fsName, err: err}
}

func resolveRoot(root string) (string, error) {
	// The mount table holds absolute mount points, so a relative root must be
	// absolute before it is compared with them.
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

// bounded runs check off the session goroutine and waits for it at most
// timeout, or until the session ends: a stat on a hard-mounted share whose
// server is down never returns, and the session must stay responsive. busy
// keeps at most one such check per library; while it hangs, later calls give
// up at once. Like the pollers, the check is not in the shutdown wait group.
func bounded[T any](ctx context.Context, busy *atomic.Bool, timeout time.Duration, check func() T) (T, bool) {
	var zero T
	if !busy.CompareAndSwap(false, true) {
		return zero, false
	}
	done := make(chan T, 1)
	go func() {
		result := check()
		// Free the library before answering: a caller that acts on the
		// answer, such as a layout check re-arming the library, runs the
		// next bounded check at once.
		busy.Store(false)
		done <- result
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case result := <-done:
		return result, true
	case <-timer.C:
	case <-ctx.Done():
	}
	return zero, false
}

type watchedDir struct {
	path      string
	libraries map[*library]bool
}

type deferral struct {
	library    *library
	rel        string
	eligibleAt time.Time
}

// session is one enabled period of the watcher. A single goroutine owns all
// of its state; the inotify reader and the pollers only hand it events.
type session struct {
	deps    Dependencies
	timing  timing
	polling pollTiming
	open    func(*sync.WaitGroup) (notifier, error)
	// resolve resolves a library root wherever the session or its pollers
	// look at one.
	resolve  func(root string) (string, error)
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	reloads  chan struct{}
	notifier notifier
	raw      <-chan rawEvent

	libraries []*library
	watches   map[int32]*watchedDir
	dirs      map[string]int32
	polls     chan pollReport

	deferredMu     sync.Mutex
	deferrals      []deferral
	deferredSignal chan struct{}
}

func startSession(deps Dependencies, t timing, polling pollTiming, open func(*sync.WaitGroup) (notifier, error), resolve func(string) (string, error)) *session {
	ctx, cancel := context.WithCancel(deps.Context)
	s := &session{
		deps: deps, timing: t, polling: polling, open: open, resolve: resolve, ctx: ctx, cancel: cancel,
		done: make(chan struct{}), reloads: make(chan struct{}, 1),
		watches: make(map[int32]*watchedDir), dirs: make(map[string]int32),
		polls: make(chan pollReport), deferredSignal: make(chan struct{}, 1),
	}
	for _, configured := range deps.Libraries {
		s.libraries = append(s.libraries, &library{Library: configured})
	}
	deps.Wait.Go(func() {
		defer close(s.done)
		s.run()
	})
	return s
}

// stop ends the session and waits until its watches are released.
func (s *session) stop() {
	s.cancel()
	<-s.done
}

// reload re-reads the library roots; it never blocks.
func (s *session) reload() {
	select {
	case s.reloads <- struct{}{}:
	default:
	}
}

func (s *session) now() time.Time {
	return s.deps.Now()
}

func (s *session) run() {
	defer s.release()
	n, err := s.open(s.deps.Wait)
	switch {
	case err == nil:
		s.notifier, s.raw = n, n.events()
	case !errors.Is(err, errNotifierUnsupported):
		s.deps.Logger.Warn("library watcher cannot use inotify; polling every library", "error", err)
	}
	for _, lib := range s.libraries {
		s.arm(lib)
	}
	s.deps.Logger.Info("library watcher enabled")
	s.dispatch(s.now())
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		s.resetTimer(timer)
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.raw:
			if !ok {
				s.notifierFailed()
				break
			}
			s.handle(event)
		case report := <-s.polls:
			s.handlePoll(report)
		case <-s.deferredSignal:
			s.drainDeferrals()
		case <-s.reloads:
			s.rearmChanged()
		case <-timer.C:
		}
		s.dispatch(s.now())
	}
}

func (s *session) release() {
	for _, lib := range s.libraries {
		if lib.stopPoll != nil {
			lib.stopPoll()
		}
	}
	if s.notifier != nil {
		err := s.notifier.close()
		if err != nil {
			s.deps.Logger.Warn("close inotify", "error", err)
		}
	}
	s.deps.Logger.Info("library watcher disabled")
}

func (s *session) resetTimer(timer *time.Timer) {
	next := time.Time{}
	for _, lib := range s.libraries {
		for _, at := range s.wakeTimes(lib) {
			if !at.IsZero() && (next.IsZero() || at.Before(next)) {
				next = at
			}
		}
	}
	if next.IsZero() {
		timer.Reset(time.Hour)
		return
	}
	timer.Reset(max(next.Sub(s.now()), 0))
}

func (s *session) wakeTimes(lib *library) []time.Time {
	if lib.mode == modeIdle {
		return nil
	}
	var times []time.Time
	// The poller watches its own layout.
	if lib.mode == modeInotify {
		times = append(times, lib.checkAt)
	}
	if lib.offline {
		return times
	}
	times = append(times, lib.pending.due())
	if lib.catchUp {
		times = append(times, lib.retryAt)
	}
	return times
}

// configuredRoot is the library directory as the settings hold it now; empty
// when unset.
func configuredRoot(lib *library) string {
	directory := lib.Directory()
	if !directory.Valid {
		return ""
	}
	return directory.String
}

// arm sets a library up from its configured root: inotify where events can
// be trusted, the poller otherwise, and a catch-up walk once it is present.
func (s *session) arm(lib *library) {
	lib.root = configuredRoot(lib)
	lib.pending = newPending(s.timing)
	lib.catchUp, lib.retryAt, lib.offline = false, time.Time{}, false
	lib.layout = layout{root: lib.root}
	lib.mode = modeIdle
	if lib.root == "" {
		return
	}
	lib.mode = modeInotify
	root := lib.root
	probe, answered := bounded(s.ctx, &lib.probing, s.timing.probe, func() rootProbe { return probeRoot(s.resolve, root) })
	if !answered {
		s.notAnswered(lib)
		return
	}
	if probe.err != nil {
		s.goOffline(lib, "the library directory is unavailable", probe.err)
		return
	}
	lib.resolved = probe.resolved
	lib.layout.root = probe.resolved
	absolute, err := filepath.Abs(lib.root)
	lib.linked = err != nil || absolute != probe.resolved
	poll := s.notifier == nil || probe.fsName != ""
	if !poll {
		err := s.watchLibrary(lib)
		poll = errors.Is(err, errNetworkMount) || watchLimitReached(err)
		if poll {
			s.unwatchTree(lib, lib.resolved)
		}
		if watchLimitReached(err) {
			s.deps.Logger.Warn("inotify watch limit reached; polling the library instead. Raise fs.inotify.max_user_watches to watch it with events",
				"library", lib.Name, "error", err)
		}
		if err != nil && !poll {
			s.goOffline(lib, "the library directory could not be watched", err)
			return
		}
	}
	if poll {
		s.startPoll(lib)
		s.deps.Logger.Info("library watcher armed", "library", lib.Name, "backend", lib.mode.String(), "filesystem", probe.fsName)
		return
	}
	lib.catchUp = true
	lib.checkAt = s.now().Add(s.polling.recheck)
	s.deps.Logger.Info("library watcher armed", "library", lib.Name, "backend", lib.mode.String(), "directories", s.countWatches(lib), "mounts", len(lib.layout.mounts))
}

func (s *session) disarm(lib *library) {
	switch lib.mode {
	case modeInotify:
		if lib.resolved != "" {
			s.unwatchTree(lib, lib.resolved)
		}
	case modePoll:
		lib.stopPoll()
		lib.stopPoll, lib.poller = nil, nil
	}
	lib.mode, lib.offline, lib.catchUp = modeIdle, false, false
	lib.pending = newPending(s.timing)
}

func (s *session) rearmChanged() {
	for _, lib := range s.libraries {
		changed := configuredRoot(lib) != lib.root
		// Saving the settings again also re-arms a library that stayed
		// offline, so a layout that changed for good can be accepted.
		if changed || lib.offline {
			s.disarm(lib)
			s.arm(lib)
		}
	}
}

func (s *session) watchLibrary(lib *library) error {
	recorded, err := recordRoot(lib.resolved)
	if err != nil {
		return err
	}
	lib.layout = recorded
	_, err = s.watchTree(lib, lib.resolved, false, nil)
	return err
}

// watchTree adds a watch to every directory at or beneath directory that the
// scanner walks, each before it is listed so no entry created meanwhile is
// missed. With collect set it returns the media files it passed, for a
// directory that appeared with files already in it; reached, when set,
// receives every watch the walk added or found.
func (s *session) watchTree(lib *library, directory string, collect bool, reached map[int32]bool) ([]string, error) {
	var files []string
	collected := &files
	if !collect {
		collected = nil
	}
	err := walkLibraryDirs(lib.resolved, directory, lib.AllowDirectory, lib.Extensions, collected, func(path string, _ os.FileInfo, mount bool) error {
		if mount {
			fsName, err := networkFilesystem(path)
			if err == nil && fsName != "" {
				return errNetworkMount
			}
			lib.layout.mounts[path] = true
		}
		return s.addWatch(lib, path, reached)
	})
	return files, err
}

func (s *session) addWatch(lib *library, path string, reached map[int32]bool) error {
	wd, err := s.notifier.add(path)
	raced := errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
	if raced {
		return nil
	}
	// The scanner walk skips a directory it cannot read, so the watcher does
	// too rather than pausing the whole library.
	unreadable := errors.Is(err, fs.ErrPermission)
	if unreadable {
		s.deps.Logger.Warn("library watcher cannot read a directory; changes inside it need a library scan",
			"library", lib.Name, "path", path, "error", err)
		return nil
	}
	if err != nil {
		return err
	}
	watched, ok := s.watches[wd]
	if !ok {
		watched = &watchedDir{path: path, libraries: make(map[*library]bool)}
		s.watches[wd] = watched
	}
	if watched.path != path {
		// The kernel answers a directory renamed while its events were lost
		// with its existing watch; its events now belong to the new path.
		if s.dirs[watched.path] == wd {
			delete(s.dirs, watched.path)
		}
		watched.path = path
	}
	watched.libraries[lib] = true
	s.dirs[path] = wd
	if reached != nil {
		reached[wd] = true
	}
	return nil
}

// unwatchTree drops the library's watches at or beneath directory; a watch
// another library still uses stays.
func (s *session) unwatchTree(lib *library, directory string) {
	for path, wd := range s.dirs {
		if !scanner.PathWithin(directory, path) {
			continue
		}
		watched := s.watches[wd]
		if watched == nil {
			delete(s.dirs, path)
			continue
		}
		s.releaseWatch(lib, wd, watched)
		if s.watches[wd] == nil {
			delete(s.dirs, path)
		}
	}
}

// releaseWatch drops the library's use of a watch, and the watch itself once
// no library uses it.
func (s *session) releaseWatch(lib *library, wd int32, watched *watchedDir) {
	delete(watched.libraries, lib)
	if len(watched.libraries) > 0 {
		return
	}
	// The kernel already dropped the watch of a deleted directory, so
	// removing it again fails harmlessly.
	_ = s.notifier.remove(wd)
	delete(s.watches, wd)
	if s.dirs[watched.path] == wd {
		delete(s.dirs, watched.path)
	}
}

// rewatchLibrary walks an inotify library again after lost events: it adds
// the watches for directories it missed, follows directories renamed
// meanwhile to their new paths, and releases the watches of directories the
// walk no longer reaches, such as one moved out of the library.
func (s *session) rewatchLibrary(lib *library) error {
	stale := make(map[int32]bool)
	for wd, watched := range s.watches {
		if watched.libraries[lib] {
			stale[wd] = true
		}
	}
	reached := make(map[int32]bool)
	_, err := s.watchTree(lib, lib.resolved, false, reached)
	if err != nil {
		return err
	}
	for wd := range stale {
		watched := s.watches[wd]
		if !reached[wd] && watched != nil {
			s.releaseWatch(lib, wd, watched)
		}
	}
	return nil
}

func (s *session) countWatches(lib *library) int {
	count := 0
	for _, watched := range s.watches {
		if watched.libraries[lib] {
			count++
		}
	}
	return count
}

func (s *session) forget(wd int32) {
	watched, ok := s.watches[wd]
	if !ok {
		return
	}
	delete(s.watches, wd)
	if s.dirs[watched.path] == wd {
		delete(s.dirs, watched.path)
	}
}

func (s *session) goOffline(lib *library, reason string, err error) {
	if lib.offline {
		return
	}
	if lib.mode == modeInotify && lib.resolved != "" {
		s.unwatchTree(lib, lib.resolved)
	}
	lib.offline, lib.catchUp = true, false
	lib.checkAt = s.now().Add(s.polling.recheck)
	lib.pending = newPending(s.timing)
	args := []any{"library", lib.Name, "reason", reason}
	if err != nil {
		args = append(args, "error", err)
	}
	s.deps.Logger.Warn("library watcher paused the library; nothing is removed until it is back", args...)
}

// notAnswered pauses a library whose bounded check gave up, unless it gave up
// because the session is ending.
func (s *session) notAnswered(lib *library) {
	if s.ctx.Err() != nil {
		return
	}
	s.goOffline(lib, "the library directory did not answer", nil)
}

// checkLayout looks at an inotify library's layout: a missing part pauses
// it, and a root that changed target or identity, or that is back after an
// outage, is armed again, which walks the whole library once.
func (s *session) checkLayout(lib *library) {
	lib.checkAt = s.now().Add(s.polling.recheck)
	// The check reads its own copy: a check that hangs outlives this call,
	// and the session may change the layout meanwhile.
	recorded := lib.layout
	recorded.mounts = maps.Clone(recorded.mounts)
	root, resolved := lib.root, lib.resolved
	type layoutState struct{ present, changed bool }
	state, answered := bounded(s.ctx, &lib.probing, s.timing.probe, func() layoutState {
		current, err := s.resolve(root)
		if err != nil {
			return layoutState{}
		}
		if current != resolved {
			return layoutState{present: true, changed: true}
		}
		present, changed := recorded.check()
		return layoutState{present, changed}
	})
	if !answered {
		s.notAnswered(lib)
		return
	}
	present, changed := state.present, state.changed
	if !present {
		if !lib.offline {
			s.goOffline(lib, "the library directory or a mount inside it is missing", nil)
		}
		return
	}
	if !lib.offline && !changed {
		return
	}
	s.deps.Logger.Info("library watcher re-arming the library", "library", lib.Name, "after_outage", lib.offline)
	s.disarm(lib)
	s.arm(lib)
}

func (s *session) startPoll(lib *library) {
	lib.mode = modePoll
	lib.generation++
	ctx, cancel := context.WithCancel(s.ctx)
	lib.stopPoll = cancel
	p := &poller{
		library: lib, generation: lib.generation, root: lib.resolved, configuredRoot: lib.root, resolve: s.resolve, allow: lib.AllowDirectory,
		extensions: lib.Extensions, timing: s.polling, now: s.deps.Now, layout: layout{root: lib.resolved},
		resettleSignal: make(chan struct{}, 1),
	}
	lib.poller = p
	// Not in the shutdown wait group: a hard-mounted share whose server is
	// down can block a stat forever, and shutdown must not wait on it. The
	// poller only reads the filesystem and exits once ctx ends.
	go p.run(ctx, s.polls)
}

func (s *session) handlePoll(report pollReport) {
	lib := report.library
	stale := lib.mode != modePoll || report.generation != lib.generation
	if stale {
		return
	}
	if report.retargeted {
		s.disarm(lib)
		s.arm(lib)
		return
	}
	now := s.now()
	switch {
	case report.offline:
		s.goOffline(lib, "the share is unavailable", nil)
	case report.online:
		lib.offline, lib.catchUp = false, true
	}
	for _, change := range report.changes {
		lib.pending.note(change.rel, change.kind, now, change.kind == changeSettled)
	}
}

func (s *session) handle(event rawEvent) {
	if event.op&opOverflow != 0 {
		s.overflow()
		return
	}
	watched, ok := s.watches[event.wd]
	if !ok {
		return
	}
	libraries := make([]*library, 0, len(watched.libraries))
	for lib := range watched.libraries {
		libraries = append(libraries, lib)
	}
	lost := event.op&opUnmount != 0
	if lost {
		for _, lib := range libraries {
			s.goOffline(lib, "a filesystem inside the library was unmounted", nil)
		}
		return
	}
	self := event.op&(opDeleteSelf|opMoveSelf|opIgnored) != 0
	if self {
		for _, lib := range libraries {
			if watched.path == lib.resolved {
				s.goOffline(lib, "the library directory was removed or moved", nil)
			}
		}
		if event.op&opIgnored != 0 {
			s.forget(event.wd)
		}
		return
	}
	path := filepath.Join(watched.path, event.name)
	for _, lib := range libraries {
		s.apply(lib, event, path)
	}
}

func (s *session) apply(lib *library, event rawEvent, path string) {
	if lib.offline || lib.mode != modeInotify {
		return
	}
	rel, ok := relative(lib.resolved, path)
	if !ok {
		return
	}
	now := s.now()
	if event.isDir {
		allowed := lib.AllowDirectory == nil || lib.AllowDirectory(lib.resolved, path)
		if !allowed {
			return
		}
		switch {
		case event.op&(opCreate|opMovedTo) != 0:
			files, err := s.watchTree(lib, path, true, nil)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				s.fallBackToPolling(lib, err)
				return
			}
			for _, file := range files {
				fileRel, ok := relative(lib.resolved, file)
				if ok {
					lib.pending.note(fileRel, changeSettled, now, false)
				}
			}
		case event.op&(opDelete|opMovedFrom) != 0:
			s.unwatchTree(lib, path)
			lib.pending.note(rel, changeRemoved, now, false)
		}
		return
	}
	if !lib.Extensions[helpers.FileExtension(path)] {
		return
	}
	switch {
	case event.op&(opDelete|opMovedFrom) != 0:
		lib.pending.note(rel, changeRemoved, now, false)
	case event.op&opWrite != 0:
		lib.pending.note(rel, changeWritten, now, false)
	case event.op&(opCreate|opCloseWrite|opMovedTo) != 0:
		lib.pending.note(rel, changeSettled, now, false)
	case event.op&opAttrib != 0:
		lib.pending.note(rel, changeAttributes, now, false)
	}
}

// fallBackToPolling switches a library to the poller after its watches
// failed: the watch limit, or a network mount that appeared inside it.
func (s *session) fallBackToPolling(lib *library, err error) {
	s.deps.Logger.Warn("library watcher switched the library to polling", "library", lib.Name, "error", err)
	s.unwatchTree(lib, lib.resolved)
	lib.pending = newPending(s.timing)
	lib.catchUp = false
	s.startPoll(lib)
}

// overflow handles a full kernel queue: events were lost, so every inotify
// library rebuilds its watches from a walk and catches up with a library
// scan.
func (s *session) overflow() {
	s.deps.Logger.Warn("library watcher lost events; catching up with a library scan")
	for _, lib := range s.libraries {
		if lib.mode != modeInotify || lib.offline {
			continue
		}
		err := s.rewatchLibrary(lib)
		if err != nil {
			s.fallBackToPolling(lib, err)
			continue
		}
		lib.catchUp, lib.retryAt = true, time.Time{}
	}
}

// notifierFailed moves every inotify library to the poller when the kernel
// instance stops delivering events.
func (s *session) notifierFailed() {
	s.deps.Logger.Warn("library watcher stopped receiving inotify events; polling every library", "error", s.notifier.err())
	_ = s.notifier.close()
	s.notifier, s.raw = nil, nil
	s.watches, s.dirs = make(map[int32]*watchedDir), make(map[string]int32)
	for _, lib := range s.libraries {
		if lib.mode == modeInotify {
			lib.mode = modeIdle
			s.arm(lib)
		}
	}
}

func (s *session) onDeferred(lib *library) func(string, time.Time) {
	return func(rel string, eligibleAt time.Time) {
		s.deferredMu.Lock()
		s.deferrals = append(s.deferrals, deferral{library: lib, rel: rel, eligibleAt: eligibleAt})
		s.deferredMu.Unlock()
		select {
		case s.deferredSignal <- struct{}{}:
		default:
		}
	}
}

func (s *session) drainDeferrals() {
	s.deferredMu.Lock()
	deferrals := s.deferrals
	s.deferrals = nil
	s.deferredMu.Unlock()
	now := s.now()
	for _, d := range deferrals {
		lib := d.library
		switch {
		case lib.mode == modeIdle || lib.offline:
		case lib.mode == modePoll:
			// The scanner saw the file change after the poller had settled
			// it; a poll cannot see a writer close a file, so the poller must
			// watch it hold still again before it dispatches.
			lib.poller.resettle(d.rel)
		default:
			lib.pending.deferred(d.rel, d.eligibleAt, now)
		}
	}
}

func (s *session) inspector(lib *library) inspectFunc {
	if lib.mode == modePoll {
		// Every pending file of a polled library came from the poller, which
		// settled it; deferred files go back to it. A stat here could block
		// on a share that hangs.
		return func(string) (time.Time, bool) { return time.Time{}, true }
	}
	return func(rel string) (time.Time, bool) {
		info, err := os.Stat(filepath.Join(lib.resolved, rel))
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			return time.Time{}, false
		}
		if err != nil {
			// The scanner reports a file it cannot read.
			return time.Time{}, true
		}
		return scanner.FingerprintOf(info).ChangedAt(), true
	}
}

func (s *session) dispatch(now time.Time) {
	for _, lib := range s.libraries {
		if lib.mode == modeIdle {
			continue
		}
		checkDue := lib.mode == modeInotify && !now.Before(lib.checkAt)
		if checkDue {
			s.checkLayout(lib)
		}
		if lib.offline || lib.mode == modeIdle {
			continue
		}
		catchUpDue := lib.catchUp && !now.Before(lib.retryAt)
		if catchUpDue {
			s.startCatchUp(lib, now)
		}
		due := lib.pending.due()
		if due.IsZero() || now.Before(due) {
			continue
		}
		retargeted := lib.mode == modeInotify && s.retargeted(lib)
		if retargeted {
			continue
		}
		items := lib.pending.take(now, s.inspector(lib))
		if len(items) == 0 {
			continue
		}
		s.startChanges(lib, items, now)
	}
}

// retargeted resolves a symlinked inotify root again before a run of changes
// dispatches: the changes were seen under the target it had, the layout check
// that notices a retarget runs only every 30 seconds, and the scanner would
// apply them under the new target. A changed target arms the library again,
// which drops them and walks it once; a root that does not resolve or answer
// pauses it.
func (s *session) retargeted(lib *library) bool {
	if !lib.linked {
		return false
	}
	root, resolved := lib.root, lib.resolved
	probe, answered := bounded(s.ctx, &lib.probing, s.timing.probe, func() rootProbe {
		current, err := s.resolve(root)
		return rootProbe{resolved: current, err: err}
	})
	if !answered {
		s.notAnswered(lib)
		return true
	}
	if probe.err != nil {
		s.goOffline(lib, "the library directory is unavailable", probe.err)
		return true
	}
	if probe.resolved == resolved {
		return false
	}
	s.deps.Logger.Info("library watcher re-arming the library", "library", lib.Name, "reason", "the root's symlink target changed")
	s.disarm(lib)
	s.arm(lib)
	return true
}

func (s *session) startCatchUp(lib *library, now time.Time) {
	result := lib.StartChanges(scanner.Changes{CatchUp: true, OnDeferred: s.onDeferred(lib)})
	switch result.Status {
	case scanner.StartAlreadyRunning:
		lib.retryAt = now.Add(s.timing.retry)
	case scanner.StartNotConfigured:
		lib.catchUp = false
	default:
		// Started, or queued behind a run of changes: it walks either way.
		lib.catchUp = false
		s.deps.Logger.Info("library watcher walking the whole library", "library", lib.Name)
	}
}

func (s *session) startChanges(lib *library, items []batchItem, now time.Time) {
	paths := make([]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.rel)
	}
	result := lib.StartChanges(scanner.Changes{Paths: paths, OnDeferred: s.onDeferred(lib)})
	switch result.Status {
	case scanner.StartAlreadyRunning:
		for _, item := range items {
			lib.pending.hold(item.rel, item.entry, now.Add(s.timing.retry))
		}
	case scanner.StartNotConfigured:
	default:
		s.deps.Logger.Info("library watcher applying changes", "library", lib.Name, "paths", len(paths), "backend", lib.mode.String())
	}
}
