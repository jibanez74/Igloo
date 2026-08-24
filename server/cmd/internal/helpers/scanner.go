package helpers

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

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

// WalkMediaLibrary walks root, invoking onFile for each regular file whose
// extension is in validExts. Per-entry errors are wrapped with context and
// passed to onError (so the caller can log and count them) and walking
// continues; an error reading the root itself aborts the walk and is returned.
func WalkMediaLibrary(
	root string,
	validExts map[string]bool,
	onError func(error),
	onFile func(ScanFile) error,
) error {
	return walkMediaLibrary(context.Background(), root, validExts, onError, onFile)
}

// WalkMediaLibraryContext walks a media library until it completes or ctx is
// canceled. It otherwise has the same per-entry error behavior as
// WalkMediaLibrary.
func WalkMediaLibraryContext(
	ctx context.Context,
	root string,
	validExts map[string]bool,
	onError func(error),
	onFile func(ScanFile) error,
) error {
	return walkMediaLibrary(ctx, root, validExts, onError, onFile)
}

func walkMediaLibrary(
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

		ext := GetFileExtension(path)
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
