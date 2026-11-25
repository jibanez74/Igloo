package helpers

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

// OverlappingLibrary reports the first of the library roots that dir is,
// contains or lies inside. Symlinks are resolved on both sides, since a
// folder that links into a library is that library. A dir that cannot be
// resolved overlaps and reports its error; a root that cannot be resolved
// overlaps too. Code that writes and deletes under the static directory never
// does so in an overlapping folder: a library's own files could be taken for
// its copies.
func OverlappingLibrary(dir string, roots []string) (string, bool) {
	resolvedDir, err := ResolvedPath(dir)
	if err != nil {
		return err.Error(), true
	}

	found, overlaps := "", false
	for _, library := range roots {
		libraryPath, err := ResolvedPath(library)
		if err != nil || PathsOverlap(resolvedDir, libraryPath) {
			found, overlaps = library, true
		}
	}

	return found, overlaps
}

// ResolvedPath is the absolute path p leads to with every symlink in it
// resolved. A path that does not exist yet resolves through its nearest
// existing ancestor, since a folder created there lands wherever that
// ancestor leads.
func ResolvedPath(p string) (string, error) {
	current, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(append([]string{resolved}, missing...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}

// PathsOverlap reports whether one of two clean absolute paths is or lies
// beneath the other, the filesystem root included.
func PathsOverlap(a, b string) bool {
	return isWithin(a, b) || isWithin(b, a)
}

func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	isOutside := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
	return !isOutside
}
