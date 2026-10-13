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
	"sync/atomic"
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
	// Callers only branch on the two failure statuses and let the success case
	// fall through, so StartStarted is never named outside tests. It is the
	// zero value and cannot be dropped.
	StartStarted StartStatus = iota
	StartNotConfigured
	StartAlreadyRunning
)

// StartResult records the observed directory and start outcome.
type StartResult struct {
	Directory string
	Status    StartStatus
}

// Launcher admits one scan goroutine at a time and tracks it in the shutdown
// wait group, so every scanner shares one start lifecycle.
type Launcher struct {
	Wait    *sync.WaitGroup
	running atomic.Bool
}

// Launch runs run(directory) on a new goroutine when directory is configured
// and no scan is in progress. begin runs before the goroutine starts so a
// status poll right after the request already sees the run.
func (l *Launcher) Launch(directory sql.NullString, begin func(), run func(directory string)) StartResult {
	result := StartResult{Directory: directory.String}
	if !directory.Valid || directory.String == "" {
		result.Status = StartNotConfigured
		return result
	}
	if !l.running.CompareAndSwap(false, true) {
		result.Status = StartAlreadyRunning
		return result
	}
	begin()
	l.Wait.Add(1)
	go func() {
		defer l.Wait.Done()
		defer l.running.Store(false)
		run(directory.String)
	}()
	return result
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
