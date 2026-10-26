// Package watch applies filesystem changes under the library roots through
// the scanners while the library watcher setting is on.
//
// The watcher only supplies hints. What is new, changed, missing or safe to
// delete is still decided by the scanner's own checks, so a missed or
// repeated event costs a stat at worst. Linux libraries on local disks use
// inotify; network shares, FUSE mounts and every library on macOS are
// polled. While the setting is off nothing runs: no goroutine, no inotify
// instance, no timer.
package watch

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"igloo/cmd/internal/logger"
	"igloo/cmd/internal/scanner"
)

// Library is one library root the watcher follows and the scanner it feeds.
type Library struct {
	Name       string
	Directory  func() sql.NullString
	Extensions map[string]bool
	// AllowDirectory limits the watched directories to those the scanner
	// walks; nil allows every directory.
	AllowDirectory func(root, path string) bool
	StartChanges   func(scanner.Changes) scanner.StartResult
}

type Dependencies struct {
	// Context ends every session; it is the scan context, so shutdown and a
	// failed startup stop the watcher with the scans.
	Context   context.Context
	Wait      *sync.WaitGroup
	Logger    logger.LoggerInterface
	Libraries []Library
	Now       func() time.Time
}

// Watcher starts and stops the watcher session as the setting changes.
type Watcher struct {
	deps    Dependencies
	timing  timing
	polling pollTiming
	// open opens the kernel instance and resolve resolves a library root, the
	// one filesystem call that can hang on a share; tests substitute
	// scripted ones.
	open    func(*sync.WaitGroup) (notifier, error)
	resolve func(root string) (string, error)

	mu      sync.Mutex
	session *session
}

// New builds a stopped watcher; it starts nothing.
func New(deps Dependencies) *Watcher {
	if deps.Context == nil {
		deps.Context = context.Background()
	}
	if deps.Wait == nil {
		deps.Wait = &sync.WaitGroup{}
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Watcher{deps: deps, timing: defaultTiming, polling: defaultPollTiming, open: newNotifier, resolve: resolveRoot}
}

// Sync brings the watcher in line with the setting. Turning it on arms every
// library and walks each once to catch up; turning it off returns only after
// the session released its watches. While on, Sync re-reads the library
// roots, re-arming any that changed or stayed offline. Once the scan context
// has ended, as at shutdown, it only stops.
func (w *Watcher) Sync(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !enabled || w.deps.Context.Err() != nil {
		if w.session != nil {
			w.session.stop()
			w.session = nil
		}
		return
	}
	if w.active() {
		w.session.reload()
		return
	}
	w.session = startSession(w.deps, w.timing, w.polling, w.open, w.resolve)
}

// active reports a session that has not ended; the caller holds w.mu.
func (w *Watcher) active() bool {
	return w.session != nil && w.session.ctx.Err() == nil
}
