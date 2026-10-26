package watch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	// table holds the mount points beneath the root, relative to it, that
	// mountPoints listed when the layout was recorded or that its walk passed;
	// tableRead is unset when the mount table could not be read then.
	table       []string
	tableRead   bool
	mountPoints func() ([]string, error)
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

// mountAppeared reports a filesystem mounted since the layout was recorded on
// a directory the library's walk reached, or on one beneath such a directory,
// such as a disk that came up late after a power cut: mounting over a watched
// directory sends no event, and its watch no longer sees the directories the
// disk brings. Mounts the walk never reaches, such as the snapshots ZFS
// mounts beneath its hidden .zfs directory, are ignored. It reads the mount
// table, so it never touches a share whose server is down.
func (l layout) mountAppeared(walked func(directory string) bool) bool {
	if l.mountPoints == nil || !l.tableRead {
		return false
	}
	points, err := l.mountPoints()
	if err != nil {
		return false
	}
	for _, point := range scanner.MountPointsBeneath(points, l.root) {
		known := slices.Contains(l.table, point)
		path := filepath.Join(l.root, point)
		reached := point == "." || walked(path) || walked(filepath.Dir(path))
		if !known && reached {
			return true
		}
	}
	return false
}

// noteWalkedMounts adds the mount points the walk passed to the table, such
// as automounts the walk itself set off after the table was read.
func (l *layout) noteWalkedMounts() {
	for mount := range l.mounts {
		point, err := filepath.Rel(l.root, mount)
		known := slices.Contains(l.table, point)
		if err == nil && !known {
			l.table = append(l.table, point)
		}
	}
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

// recordRoot fills the root half of a layout from the directory as it is now,
// with the mount points beneath it that mountPoints lists.
func recordRoot(root string, mountPoints func() ([]string, error)) (layout, error) {
	info, err := os.Stat(root)
	if err != nil {
		return layout{}, err
	}
	recorded := layout{root: root, rootID: identityOf(info), rootMount: mounted(root, info), mounts: make(map[string]bool), mountPoints: mountPoints}
	if mountPoints != nil {
		points, err := mountPoints()
		recorded.tableRead = err == nil
		recorded.table = scanner.MountPointsBeneath(points, root)
	}
	return recorded, nil
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
