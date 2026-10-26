package watch

import (
	"io/fs"
	"os"
	"path/filepath"

	"igloo/cmd/internal/helpers"
)

// walkLibraryDirs visits every directory at or beneath directory that the
// scanner walks under root, with its stat and whether it is a mount point: on
// another device than its parent. With files set it also collects the media
// files it passes. An error reading directory itself, or from visit, stops
// the walk; an entry below directory that cannot be read is skipped.
func walkLibraryDirs(
	root, directory string,
	allow func(root, path string) bool,
	extensions map[string]bool,
	files *[]string,
	visit func(path string, info os.FileInfo, mount bool) error,
) error {
	devices := make(map[string]string)
	return filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == directory {
				return err
			}
			return nil
		}
		if !entry.IsDir() {
			isMedia := files != nil && extensions[helpers.FileExtension(path)]
			if isMedia {
				*files = append(*files, path)
			}
			return nil
		}
		allowed := path == root || allow == nil || allow(root, path)
		if !allowed {
			return fs.SkipDir
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil
		}
		device := identityOf(info).device
		devices[path] = device
		mount := false
		if path != root {
			parent, known := devices[filepath.Dir(path)]
			if !known {
				// The first directory of a subtree walk: its parent was not
				// visited, so stat it.
				parentInfo, statErr := os.Stat(filepath.Dir(path))
				if statErr == nil {
					parent, known = identityOf(parentInfo).device, true
				}
			}
			mount = known && parent != device
		}
		return visit(path, info, mount)
	})
}
