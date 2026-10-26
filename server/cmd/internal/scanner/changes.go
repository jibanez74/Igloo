package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"igloo/cmd/internal/helpers"
)

// ChangeQuietPeriod replaces FileQuietPeriod in a run of watcher-detected
// changes: the watcher only dispatches a file once its writer has closed it,
// and Validate still rejects any write that lands while it is processed.
const ChangeQuietPeriod = 5 * time.Second

// Changes scopes a run to what the library watcher saw change.
type Changes struct {
	// Paths are relative to the library root. A path that no longer exists
	// removes the catalog rows at or beneath it.
	Paths []string
	// CatchUp walks the whole library instead, after the watcher could not
	// see what changed. It keeps FileQuietPeriod and refuses cleanup when the
	// walk finds nothing, since an empty root is more likely a dropped mount.
	CatchUp bool
	// OnDeferred hands back each file the run deferred, with the time it
	// becomes eligible when known, so the watcher can retry it. It runs on the
	// scan goroutine and must not block.
	OnDeferred func(path string, eligibleAt time.Time)
}

// A nil *Changes is a library scan: requested by an admin or run at startup.

// Scoped reports a run limited to Paths.
func (c *Changes) Scoped() bool {
	return c != nil && !c.CatchUp
}

// Scope is what the run's status reports: a catch-up walks the whole library,
// so it reports as a library scan.
func (c *Changes) Scope() ScanScope {
	if c.Scoped() {
		return ScopeChanges
	}
	return ScopeLibrary
}

// QuietPeriod is how long a file must have been unchanged before this run
// processes it.
func (c *Changes) QuietPeriod() time.Duration {
	if c == nil || c.CatchUp {
		return FileQuietPeriod
	}
	return ChangeQuietPeriod
}

// Deferred reports a deferred file to the watcher, as a path relative to root.
func (c *Changes) Deferred(root, path string, eligibleAt time.Time) {
	if c == nil || c.OnDeferred == nil {
		return
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return
	}
	c.OnDeferred(relative, eligibleAt)
}

// DeferredReason is the issue text for a deferred file: the watcher retries
// what its runs defer, while a library scan words its own retry.
func (c *Changes) DeferredReason(libraryScan string) string {
	if c == nil {
		return libraryScan
	}
	return ReasonFileDeferredWatched
}

// Resolve joins the paths onto root, the same spelling of the root the run
// walks, so lookups in the catalog index match exactly. Paths that are empty,
// name the root itself, or escape it are dropped, as are paths beneath
// another one; the result is sorted. A run that is not Scoped has none.
func (c *Changes) Resolve(root string) []string {
	if !c.Scoped() {
		return nil
	}
	resolved := make([]string, 0, len(c.Paths))
	for _, path := range c.Paths {
		relative := filepath.Clean(path)
		if path == "" || relative == "." || escapesRoot(relative) {
			continue
		}
		resolved = append(resolved, filepath.Join(root, relative))
	}
	slices.Sort(resolved)
	resolved = slices.Compact(resolved)
	collapsed := resolved[:0]
	for _, path := range resolved {
		covered := len(collapsed) > 0 && PathWithin(collapsed[len(collapsed)-1], path)
		if !covered {
			collapsed = append(collapsed, path)
		}
	}
	return collapsed
}

// ChildBounds returns the half-open range [lower, upper) of every path
// beneath path in byte order, so a catalog query can seek its file_path index
// for the files under a changed directory.
func ChildBounds(path string) (lower, upper string) {
	return path + string(filepath.Separator), path + string(filepath.Separator+1)
}

// LoadUnder reads the catalog rows at or beneath each of paths, which
// Changes.Resolve built: query receives a path and the ChildBounds of its
// children, and convert turns its rows into the library scan's index rows.
func LoadUnder[Under, Row any](
	ctx context.Context,
	paths []string,
	query func(ctx context.Context, path, lower, upper string) ([]Under, error),
	convert func(Under) Row,
) ([]Row, error) {
	rows := make([]Row, 0)
	for _, path := range paths {
		lower, upper := ChildBounds(path)
		under, err := query(ctx, path, lower, upper)
		if err != nil {
			return nil, err
		}
		for _, row := range under {
			rows = append(rows, convert(row))
		}
	}
	return rows, nil
}

// PathWithin reports whether path is parent or lies beneath it. Both are
// clean; a parent of "/" holds every absolute path.
func PathWithin(parent, path string) bool {
	prefix := parent
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return path == parent || strings.HasPrefix(path, prefix)
}

// escapesRoot reports whether a cleaned relative path leaves its root.
func escapesRoot(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative)
}

// WalkRun walks what a run covers: the whole library for a library scan or a
// catch-up, and only paths, which Changes.Resolve built from root, for a run
// of changes. directoryFilter may be nil.
func WalkRun(
	ctx context.Context,
	root string,
	changes *Changes,
	paths []string,
	validExts map[string]bool,
	onError func(path string, err error),
	onFile func(ScanFile) error,
	directoryFilter func(string) bool,
) error {
	if changes.Scoped() {
		return walkChanges(ctx, root, paths, validExts, onError, onFile, directoryFilter)
	}
	return WalkMediaLibraryContext(ctx, root, validExts, onError, onFile, optionalFilter(directoryFilter)...)
}

// optionalFilter adapts a directory filter that may be nil to
// WalkMediaLibraryContext's variadic filters.
func optionalFilter(directoryFilter func(string) bool) []func(string) bool {
	if directoryFilter == nil {
		return nil
	}
	return []func(string) bool{directoryFilter}
}

// walkChanges is the scoped counterpart of WalkMediaLibraryContext: it visits
// only the media files at or beneath paths. A missing path visits nothing, so
// its catalog rows stay unseen. The directory filter is applied to every
// directory between root and a path, and to a directory path itself, so a
// path the full walk would never reach is skipped the same way.
func walkChanges(
	ctx context.Context,
	root string,
	paths []string,
	validExts map[string]bool,
	onError func(path string, err error),
	onFile func(ScanFile) error,
	directoryFilter func(string) bool,
) error {
	for _, path := range paths {
		err := ctx.Err()
		if err != nil {
			return err
		}
		reachable := directoryFilter == nil || ancestorsAllowed(root, path, directoryFilter)
		if !reachable {
			continue
		}
		entry, err := os.Lstat(path)
		missing := errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
		if missing {
			continue
		}
		if err != nil {
			onError(path, fmt.Errorf("failed to get file info for %s: %w", path, err))
			continue
		}
		if entry.IsDir() {
			allowed := directoryFilter == nil || directoryFilter(path)
			if !allowed {
				continue
			}
			err = walkChangedDirectory(ctx, path, validExts, onError, onFile, directoryFilter)
			if err != nil {
				return err
			}
			continue
		}
		err = visitChangedFile(path, entry, validExts, onError, onFile)
		if err != nil {
			return err
		}
	}
	return ctx.Err()
}

func ancestorsAllowed(root, path string, directoryFilter func(string) bool) bool {
	for directory := filepath.Dir(path); PathWithin(root, directory) && directory != root; directory = filepath.Dir(directory) {
		allowed := directoryFilter(directory)
		if !allowed {
			return false
		}
	}
	return true
}

// walkChangedDirectory walks one changed directory. A directory that vanished
// since it was listed is treated as removed; any other failure to read it is
// reported, and its catalog rows survive because ConfirmMissing only accepts
// nonexistence.
func walkChangedDirectory(
	ctx context.Context,
	directory string,
	validExts map[string]bool,
	onError func(path string, err error),
	onFile func(ScanFile) error,
	directoryFilter func(string) bool,
) error {
	visitErr := error(nil)
	err := WalkMediaLibraryContext(ctx, directory, validExts, onError, func(file ScanFile) error {
		visitErr = onFile(file)
		return visitErr
	}, optionalFilter(directoryFilter)...)
	if err == nil || visitErr != nil || ctx.Err() != nil {
		return err
	}
	vanished := errors.Is(err, fs.ErrNotExist)
	if !vanished {
		onError(directory, fmt.Errorf("error walking directory: %w", err))
	}
	return nil
}

func visitChangedFile(path string, entry fs.FileInfo, validExts map[string]bool, onError func(path string, err error), onFile func(ScanFile) error) error {
	ext := helpers.FileExtension(path)
	if !validExts[ext] {
		return nil
	}
	info := entry
	isSymlink := entry.Mode()&fs.ModeSymlink != 0
	if isSymlink {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			onError(path, fmt.Errorf("failed to get file info for %s: %w", path, err))
			return nil
		}
	}
	regular := info.Mode().IsRegular()
	if !regular {
		return nil
	}
	return onFile(ScanFile{Path: path, Ext: ext, Size: info.Size()})
}
