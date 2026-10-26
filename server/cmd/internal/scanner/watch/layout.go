package watch

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"igloo/cmd/internal/scanner"
)

type identity struct {
	device string
	inode  string
}

func identityOf(info os.FileInfo) identity {
	fingerprint := scanner.FingerprintOf(info)
	return identity{device: fingerprint.Device, inode: fingerprint.Inode}
}

// layout is what a library looked like when the watcher armed it: the root,
// whether it is a mount point itself, and the mount points beneath it. A
// dropped mount leaves an empty, readable directory behind, and the scanner
// would read every file under it as deleted, so the watcher dispatches
// nothing for a library while part of its layout is missing.
type layout struct {
	root      string
	rootID    identity
	rootMount bool
	mounts    map[string]bool
}

// check reports whether the layout is present, and whether the root changed
// identity since it was recorded, which calls for arming the library again: a
// filesystem mounted over it, a remount, or a recreated directory. A mount
// point counts as mounted while its device differs from its parent's; a
// remount may bring a new device number, so the old one is not compared. A
// layout that was never recorded only needs the root to exist.
func (l layout) check() (present, changed bool) {
	info, err := os.Stat(l.root)
	if err != nil || !info.IsDir() {
		return false, false
	}
	recorded := l.rootID != identity{}
	if !recorded {
		return true, false
	}
	if l.rootMount && !mounted(l.root, info) {
		return false, false
	}
	for mount := range l.mounts {
		mountInfo, err := os.Stat(mount)
		if err != nil || !mounted(mount, mountInfo) {
			return false, false
		}
	}
	return true, identityOf(info) != l.rootID
}

// mounted reports whether path, whose stat is info, sits on another device
// than its parent.
func mounted(path string, info os.FileInfo) bool {
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return false
	}
	return identityOf(parent).device != identityOf(info).device
}

// recordRoot fills the root half of a layout from the directory as it is now.
func recordRoot(root string) (layout, error) {
	info, err := os.Stat(root)
	if err != nil {
		return layout{}, err
	}
	return layout{root: root, rootID: identityOf(info), rootMount: mounted(root, info), mounts: make(map[string]bool)}, nil
}

// unavailable reports errors that mean a share or disk went away rather than
// one entry being unreadable.
func unavailable(err error) bool {
	return errors.Is(err, syscall.EIO) || errors.Is(err, syscall.ESTALE) || errors.Is(err, syscall.ENOTCONN) ||
		errors.Is(err, syscall.EHOSTDOWN) || errors.Is(err, syscall.ETIMEDOUT) || errors.Is(err, syscall.ENODEV)
}

// relative returns path relative to root when it lies beneath it.
func relative(root, path string) (string, bool) {
	if path == root || !scanner.PathWithin(root, path) {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	return rel, err == nil
}
