package scanner

import (
	"context"
	"fmt"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"maps"
	"path/filepath"
	"strings"
	"sync"
)

// BatchSize is how many files a library scan buffers before flushing them to
// the database.
const BatchSize = 54

// ScanFile is a media file queued for processing during a library scan. It is
// shared by the movie and music scanners.
type ScanFile struct {
	Path string
	Ext  string
	Size int64
}

// NormalizedScanCacheKey builds a stable, case-insensitive cache key from the
// given parts. It is shared by the movie and music scanners to key their
// in-scan lookup caches (genre tags, musician/album identities, etc.).
func NormalizedScanCacheKey(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(part)))
	}

	return strings.Join(normalized, "\x00")
}

// ScanIndexUnchanged reports whether path is present in the scan index with the
// same size, i.e. the file does not need rescanning. Keys are compared with
// filepath.Clean.
func ScanIndexUnchanged(index map[string]int64, path string, size int64) bool {
	existingSize, ok := index[filepath.Clean(path)]
	return ok && existingSize == size
}

// BuildScanIndex builds a cleaned-path -> size index from DB rows. extract pulls
// the (path, size) pair out of each row. The extract closure lives in the
// caller's package, so this helper never needs to import the database package.
func BuildScanIndex[T any](rows []T, extract func(T) (string, int64)) map[string]int64 {
	index := make(map[string]int64, len(rows))
	for _, row := range rows {
		path, size := extract(row)
		index[filepath.Clean(path)] = size
	}

	return index
}

// ScanCache is a two-level map: transaction-local entries over an optional
// read-only base layer. The scan-lifetime cache uses just the local layer; the
// per-item overlay created by Overlay starts empty and reads through to the
// scan layer. Discarding the overlay after a rolled-back transaction discards
// its entries, which would otherwise require cloning every map per item --
// O(items x library) -- and now costs only the new entries.
//
// It is shared by the movie and music scanners to memoize entity ids and
// already-written join rows within a scan.
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

func (c ScanCache[K, V]) Has(k K) bool {
	_, ok := c.Get(k)
	return ok
}

func (c ScanCache[K, V]) Set(k K, v V) {
	c.local[k] = v
}

// MergeFrom publishes another cache's local entries into this cache's local
// layer, after the transaction that wrote them committed.
func (c ScanCache[K, V]) MergeFrom(other ScanCache[K, V]) {
	maps.Copy(c.local, other.local)
}

// ScanGuard is a single-flight guard ensuring only one scan of a given kind runs
// at a time. The zero value is ready to use.
type ScanGuard struct {
	mu      sync.Mutex
	running bool
}

// TryBegin marks the guard as running and returns true, or returns false if a
// scan is already in progress.
func (g *ScanGuard) TryBegin() bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return false
	}

	g.running = true
	return true
}

// Finish clears the running flag so a subsequent TryBegin can succeed.
func (g *ScanGuard) Finish() {
	g.mu.Lock()
	g.running = false
	g.mu.Unlock()
}

// WalkMediaLibraryContext walks root until it completes or ctx is canceled,
// invoking onFile for each regular file whose extension is in validExts.
// Per-entry errors are wrapped with context and passed to onError (so the
// caller can log and count them) and walking continues; an error reading the
// root itself aborts the walk and is returned.
func WalkMediaLibraryContext(
	ctx context.Context,
	root string,
	validExts map[string]bool,
	onError func(error),
	onFile func(ScanFile) error,
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
			onError(fmt.Errorf("error walking directory: %w", err))
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		ext := helpers.GetFileExtension(path)
		if !validExts[ext] {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			onError(fmt.Errorf("failed to get file info for %s: %w", path, err))
			return nil
		}

		return onFile(ScanFile{Path: path, Ext: ext, Size: info.Size()})
	})
}
