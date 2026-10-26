package watch

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"syscall"
	"time"

	"igloo/cmd/internal/logger"
)

// earlyEventCap bounds the events held for watches a setup added but the
// session has not registered yet; past it they are treated as lost.
const earlyEventCap = 4096

// setupWatch is one directory watch a walk added.
type setupWatch struct {
	path string
	wd   int32
}

// setupResult is what a setup hands back to the session: the watches it
// added, and for an arming walk the root's recorded layout.
type setupResult struct {
	library    *library
	generation int
	// rewatch marks a walk after lost events, which keeps an armed library
	// armed; stale then holds the library's watches when it started.
	rewatch bool
	stale   map[int32]bool
	layout  layout
	watches []setupWatch
	err     error
}

// collectWatches adds a watch to every directory at or beneath directory that
// the scanner walks under root, each before it is listed so no entry created
// meanwhile is missed, and records the mount points it passes in mounts. With
// files set it also collects the media files it passes. It touches no session
// state, so a setup can run it off the session goroutine; it returns the
// watches it added even when it fails.
func collectWatches(n notifier, log logger.LoggerInterface, name, root, directory string, allow func(root, path string) bool, extensions map[string]bool, files *[]string, mounts map[string]bool) ([]setupWatch, error) {
	var watches []setupWatch
	err := walkLibraryDirs(root, directory, allow, extensions, files, func(path string, _ os.FileInfo, mount bool) error {
		if mount {
			fsName, err := networkFilesystem(path)
			if err == nil && fsName != "" {
				return errNetworkMount
			}
			mounts[path] = true
		}
		wd, err := n.add(path)
		raced := errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
		if raced {
			return nil
		}
		// The scanner walk skips a directory it cannot read, so the watcher
		// does too rather than pausing the whole library.
		unreadable := errors.Is(err, fs.ErrPermission)
		if unreadable {
			log.Warn("library watcher cannot read a directory; changes inside it need a library scan",
				"library", name, "path", path, "error", err)
			return nil
		}
		if err != nil {
			return err
		}
		watches = append(watches, setupWatch{path: path, wd: wd})
		return nil
	})
	return watches, err
}

// startSetup walks an inotify library off the session goroutine: a mount
// that hangs after the root answered must stall neither the other libraries
// nor shutdown. Like the pollers, the walk is not in the shutdown wait group;
// it only adds watches and reads the filesystem, and its result is dropped
// once the session ends. The library dispatches nothing until the result is
// applied.
func (s *session) startSetup(lib *library, rewatch bool) {
	lib.generation++
	s.setupsInFlight++
	result := setupResult{library: lib, generation: lib.generation, rewatch: rewatch}
	if rewatch {
		lib.rewatching = true
		result.stale = make(map[int32]bool)
		for wd, watched := range s.watches {
			if watched.libraries[lib] {
				result.stale[wd] = true
			}
		}
	} else {
		lib.mode = modeArming
	}
	n, log, name, root := s.notifier, s.deps.Logger, lib.Name, lib.resolved
	allow, extensions := lib.AllowDirectory, lib.Extensions
	go func() {
		result.layout, result.watches, result.err = setUp(n, log, name, root, allow, extensions, rewatch)
		select {
		case s.setups <- result:
		case <-s.ctx.Done():
		}
	}()
}

// setUp is a setup's filesystem work: the root's layout for an arming walk,
// then a watch on every directory the scanner walks.
func setUp(n notifier, log logger.LoggerInterface, name, root string, allow func(root, path string) bool, extensions map[string]bool, rewatch bool) (layout, []setupWatch, error) {
	recorded := layout{root: root, mounts: make(map[string]bool)}
	if !rewatch {
		var err error
		recorded, err = recordRoot(root)
		if err != nil {
			return recorded, nil, err
		}
	}
	watches, err := collectWatches(n, log, name, root, root, allow, extensions, nil, recorded.mounts)
	return recorded, watches, err
}

// handleSetup applies a setup's result, unless the library moved on since it
// started: re-armed, switched to the poller, paused or cleared. A stale
// result's watches are released once no other setup could share them.
func (s *session) handleSetup(result setupResult) {
	s.setupsInFlight--
	lib := result.library
	current := result.generation == lib.generation
	if result.rewatch {
		current = current && lib.mode == modeInotify && !lib.offline
	} else {
		current = current && lib.mode == modeArming
	}
	switch {
	case !current:
		s.orphan(result.watches)
	case result.rewatch:
		s.finishRewatch(lib, result)
	default:
		s.finishArm(lib, result)
	}
	s.replayEarly()
	s.releaseOrphans()
}

func (s *session) finishArm(lib *library, result setupResult) {
	lib.mode = modeInotify
	err := result.err
	poll := errors.Is(err, errNetworkMount) || watchLimitReached(err)
	if watchLimitReached(err) {
		s.deps.Logger.Warn("inotify watch limit reached; polling the library instead. Raise fs.inotify.max_user_watches to watch it with events",
			"library", lib.Name, "error", err)
	}
	if err != nil {
		s.orphan(result.watches)
		if !poll {
			s.goOffline(lib, "the library directory could not be watched", err)
			return
		}
		s.startPoll(lib)
		s.deps.Logger.Info("library watcher armed", "library", lib.Name, "backend", lib.mode.String(), "reason", err)
		return
	}
	lib.layout = result.layout
	s.register(lib, result.watches)
	lib.catchUp = true
	lib.checkAt = s.now().Add(s.polling.recheck)
	s.deps.Logger.Info("library watcher armed", "library", lib.Name, "backend", lib.mode.String(), "directories", s.countWatches(lib), "mounts", len(lib.layout.mounts))
	if lib.rewatchAgain {
		lib.rewatchAgain = false
		s.startSetup(lib, true)
	}
}

// finishRewatch applies a walk after lost events: it adds the watches for
// directories the session missed, follows directories renamed meanwhile to
// their new paths, and releases the watches of directories the walk no longer
// reached, such as one moved out of the library. A directory watched by an
// event during the walk was not among the walk's stale watches, so it stays.
func (s *session) finishRewatch(lib *library, result setupResult) {
	lib.rewatching = false
	maps.Copy(lib.layout.mounts, result.layout.mounts)
	if result.err != nil {
		s.register(lib, result.watches)
		s.fallBackToPolling(lib, result.err)
		return
	}
	reached := s.register(lib, result.watches)
	for wd := range result.stale {
		watched := s.watches[wd]
		if !reached[wd] && watched != nil {
			s.releaseWatch(lib, wd, watched)
		}
	}
	lib.catchUp, lib.retryAt = true, time.Time{}
	if lib.rewatchAgain {
		lib.rewatchAgain = false
		s.startSetup(lib, true)
	}
}

// register records the watches a walk added for lib and returns their
// descriptors.
func (s *session) register(lib *library, watches []setupWatch) map[int32]bool {
	reached := make(map[int32]bool, len(watches))
	for _, added := range watches {
		delete(s.orphans, added.wd)
		watched, ok := s.watches[added.wd]
		if !ok {
			watched = &watchedDir{path: added.path, libraries: make(map[*library]bool)}
			s.watches[added.wd] = watched
		}
		if watched.path != added.path {
			// The kernel answers a directory renamed while its events were
			// lost with its existing watch; its events now belong to the new
			// path.
			if s.dirs[watched.path] == added.wd {
				delete(s.dirs, watched.path)
			}
			watched.path = added.path
		}
		watched.libraries[lib] = true
		s.dirs[added.path] = added.wd
		reached[added.wd] = true
	}
	return reached
}

// orphan keeps the watches no library registered for release. The kernel
// hands a directory watched twice the same descriptor, so a watch another
// setup still in flight also added must outlive it.
func (s *session) orphan(watches []setupWatch) {
	for _, added := range watches {
		_, registered := s.watches[added.wd]
		if !registered {
			s.orphans[added.wd] = true
		}
	}
}

// releaseOrphans removes the orphaned watches once no setup is in flight.
func (s *session) releaseOrphans() {
	if s.setupsInFlight > 0 {
		return
	}
	for wd := range s.orphans {
		_, registered := s.watches[wd]
		if !registered && s.notifier != nil {
			// The kernel already dropped the watch of a deleted directory, so
			// removing it again fails harmlessly.
			_ = s.notifier.remove(wd)
		}
	}
	clear(s.orphans)
}

// holdEarly keeps an event for a watch the session does not know while a
// setup is in flight: the walk adds a watch before it lists the directory,
// and an entry created then must not be missed.
func (s *session) holdEarly(event rawEvent) {
	if s.setupsInFlight == 0 {
		return
	}
	if len(s.early) >= earlyEventCap {
		s.earlyLost = true
		return
	}
	s.early = append(s.early, event)
}

// replayEarly handles the held events whose watches are known now, and drops
// the rest once no setup could still register them. Held events that did not
// fit are lost events, caught up like an overflow.
func (s *session) replayEarly() {
	held := s.early
	s.early = nil
	for _, event := range held {
		_, known := s.watches[event.wd]
		switch {
		case known:
			s.handle(event)
		case s.setupsInFlight > 0:
			s.early = append(s.early, event)
		}
	}
	if s.earlyLost && s.setupsInFlight == 0 {
		s.earlyLost = false
		s.overflow()
	}
}
