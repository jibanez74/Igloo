package watch

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

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

// setupResult is what a setup hands back to the session: the root's recorded
// layout and the watches it added.
type setupResult struct {
	library    *library
	generation int
	layout     layout
	watches    []setupWatch
	err        error
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
func (s *session) startSetup(lib *library) {
	lib.generation++
	lib.mode = modeArming
	s.setupsInFlight++
	result := setupResult{library: lib, generation: lib.generation}
	n, log, name, root := s.notifier, s.deps.Logger, lib.Name, lib.resolved
	allow, extensions := lib.AllowDirectory, lib.Extensions
	go func() {
		result.layout, result.watches, result.err = setUp(n, log, name, root, allow, extensions)
		select {
		case s.setups <- result:
		case <-s.ctx.Done():
		}
	}()
}

// setUp is a setup's filesystem work: the root's layout, then a watch on
// every directory the scanner walks.
func setUp(n notifier, log logger.LoggerInterface, name, root string, allow func(root, path string) bool, extensions map[string]bool) (layout, []setupWatch, error) {
	recorded, err := recordRoot(root)
	if err != nil {
		return recorded, nil, err
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
	current := result.generation == lib.generation && lib.mode == modeArming
	if current {
		s.finishArm(lib, result)
	} else {
		s.orphan(result.watches)
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
	if lib.armAgain {
		// Events were lost while the walk ran, possibly in directories it
		// had already passed.
		s.disarm(lib)
		s.arm(lib)
	}
}

// register records the watches a walk added for lib.
func (s *session) register(lib *library, watches []setupWatch) {
	for _, added := range watches {
		delete(s.orphans, added.wd)
		watched, ok := s.watches[added.wd]
		if !ok {
			watched = &watchedDir{path: added.path, libraries: make(map[*library]bool)}
			s.watches[added.wd] = watched
		}
		if watched.path != added.path {
			// The kernel answers a directory renamed while another library
			// still watched it with that watch; its events now belong to the
			// new path.
			if s.dirs[watched.path] == added.wd {
				delete(s.dirs, watched.path)
			}
			watched.path = added.path
		}
		watched.libraries[lib] = true
		s.dirs[added.path] = added.wd
	}
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
// and a directory created then must be watched too. A file's change needs no
// replay, since every setup ends in a catch-up that walks the whole library;
// dropping them keeps a copy running during the walk from filling the buffer.
func (s *session) holdEarly(event rawEvent) {
	fileChange := !event.isDir && event.op&(opDeleteSelf|opMoveSelf|opUnmount|opIgnored) == 0
	if s.setupsInFlight == 0 || fileChange {
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
