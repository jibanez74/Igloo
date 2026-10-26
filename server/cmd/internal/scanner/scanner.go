package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BatchSize is how many files a library scan buffers before flushing them to
// the database.
const BatchSize = 54

// ScanFile is a media file queued for processing during a library scan.
type ScanFile struct {
	Path string
	Ext  string
	Size int64
}

// NormalizedScanCacheKey builds a stable, case-insensitive cache key from the
// given parts, for the scanners' in-scan lookup caches.
func NormalizedScanCacheKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(part)))
	}

	return strings.Join(normalized, "\x00")
}

// ScanCache memoizes entity ids and written join rows within a scan, as
// transaction-local entries over an optional read-only base layer. The
// per-item Overlay reads through to the scan layer, so discarding it after a
// rolled-back transaction drops only its new entries without cloning maps.
type ScanCache[K comparable, V any] struct {
	local map[K]V
	base  map[K]V // nil on the scan-lifetime cache
}

func NewScanCache[K comparable, V any]() ScanCache[K, V] {
	return ScanCache[K, V]{local: make(map[K]V)}
}

// Overlay returns an empty transaction-local layer over this cache's entries.
// Only meaningful on the scan-lifetime cache (base == nil); overlays are never
// stacked.
func (c ScanCache[K, V]) Overlay() ScanCache[K, V] {
	return ScanCache[K, V]{local: make(map[K]V), base: c.local}
}

func (c ScanCache[K, V]) Get(k K) (V, bool) {
	v, ok := c.local[k]
	if ok {
		return v, true
	}

	if c.base != nil {
		v, ok = c.base[k]
		if ok {
			return v, true
		}
	}

	var zero V
	return zero, false
}

func (c ScanCache[K, V]) Set(k K, v V) {
	c.local[k] = v
}

// MergeFrom publishes another cache's local entries into this cache's local
// layer, after the transaction that wrote them committed.
func (c ScanCache[K, V]) MergeFrom(other ScanCache[K, V]) {
	maps.Copy(c.local, other.local)
}

// StartStatus describes whether a scan goroutine was launched.
type StartStatus int

const (
	// Callers only branch on the failure and queued statuses and let the
	// success case fall through, so StartStarted is never named outside tests.
	// It is the zero value and cannot be dropped.
	StartStarted StartStatus = iota
	StartNotConfigured
	StartAlreadyRunning
	// StartQueued means a library scan was requested while a run of
	// watcher-detected changes was in progress; it starts when that run ends.
	StartQueued
)

// StartResult records the observed directory and start outcome.
type StartResult struct {
	Directory string
	Status    StartStatus
}

// Launcher admits one scan goroutine at a time and tracks it in the shutdown
// wait group, so every scanner shares one start lifecycle. Library scans and
// runs of watcher-detected changes share it, so the two never overlap and the
// scan state keeps a single owner.
type Launcher struct {
	Wait *sync.WaitGroup

	mu      sync.Mutex
	running bool
	changes bool
	queued  *launchRequest
}

type launchRequest struct {
	directory func() sql.NullString
	changes   *Changes
	begin     func(ScanScope)
	run       func(directory string, changes *Changes)
}

// Start runs run(directory, changes) on a new goroutine when the configured
// directory is set and no run is in progress. changes is nil for a library
// scan; a catch-up walks the whole library, so it starts like one. begin runs
// first, with the run's scope, so a status poll right after the request
// already sees the run. A library scan requested while a run of changes is in
// progress is queued behind it rather than refused, so a busy watcher can
// never starve a requested scan; it reads the directory again when it
// launches, so it walks the root configured then. Queued requests coalesce,
// and a catch-up replaces a queued scan, since it hands deferred files back to
// the watcher and guards an empty root. A run of changes is refused while any
// run is in progress, and the watcher retries it.
func (l *Launcher) Start(directory func() sql.NullString, changes *Changes, begin func(ScanScope), run func(directory string, changes *Changes)) StartResult {
	configured := directory()
	result := StartResult{Directory: configured.String}
	if !configured.Valid || configured.String == "" {
		result.Status = StartNotConfigured
		return result
	}
	request := &launchRequest{directory: directory, changes: changes, begin: begin, run: run}
	scoped := changes.Scoped()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		queueable := !scoped && l.changes
		if !queueable {
			result.Status = StartAlreadyRunning
			return result
		}
		catchUp := changes != nil && changes.CatchUp
		replaces := l.queued == nil || catchUp
		if replaces {
			l.queued = request
		}
		result.Status = StartQueued
		return result
	}
	l.running, l.changes = true, scoped
	l.launch(request, configured.String)
	return result
}

func (l *Launcher) launch(request *launchRequest, directory string) {
	request.begin(request.changes.Scope())
	l.Wait.Go(func() {
		request.run(directory, request.changes)
		l.finish()
	})
}

// finish launches the queued library scan, if any, with the directory
// configured now. The directory is read outside the lock: reading the
// settings must never wait on a launcher that a settings save may be calling.
func (l *Launcher) finish() {
	for {
		l.mu.Lock()
		next := l.queued
		l.queued = nil
		// Only a library scan is ever queued.
		l.changes = false
		if next == nil {
			l.running = false
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()
		configured := next.directory()
		unset := !configured.Valid || configured.String == ""
		if unset {
			// The library was cleared while its scan waited; a request queued
			// meanwhile still gets its turn.
			continue
		}
		l.mu.Lock()
		l.launch(next, configured.String)
		l.mu.Unlock()
		return
	}
}

// WalkMediaLibraryContext walks root until it completes or ctx is canceled,
// invoking onFile for each regular file whose extension is in validExts.
// Per-entry errors are wrapped with context and passed to onError with the
// entry's path (so the caller can log and report them) and walking continues;
// an error reading the root itself aborts the walk and is returned.
func WalkMediaLibraryContext(
	ctx context.Context,
	root string,
	validExts map[string]bool,
	onError func(path string, err error),
	onFile func(ScanFile) error,
	directoryFilter ...func(string) bool,
) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		contextErr := ctx.Err()
		if contextErr != nil {
			return contextErr
		}
		if err != nil {
			if path == root {
				return err
			}
			onError(path, fmt.Errorf("error walking directory: %w", err))
			return nil
		}

		if entry.IsDir() {
			if path != root && len(directoryFilter) > 0 && directoryFilter[0] != nil && !directoryFilter[0](path) {
				return fs.SkipDir
			}
			return nil
		}

		ext := helpers.FileExtension(path)
		if !validExts[ext] {
			return nil
		}

		var info fs.FileInfo
		isSymlink := entry.Type()&fs.ModeSymlink != 0
		if isSymlink {
			info, err = os.Stat(path)
		} else {
			info, err = entry.Info()
		}
		if err != nil {
			onError(path, fmt.Errorf("failed to get file info for %s: %w", path, err))
			return nil
		}

		regular := info.Mode().IsRegular()
		if !regular {
			return nil
		}

		return onFile(ScanFile{Path: path, Ext: ext, Size: info.Size()})
	})
}
