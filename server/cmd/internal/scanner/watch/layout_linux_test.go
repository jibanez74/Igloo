package watch

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

// A disk mounted beneath the root after the layout was recorded, as a disk
// that came up late does, calls for arming the library again; the mounts the
// table already listed do not.
func TestLayoutNoticesAFilesystemMountedBeneathTheRoot(t *testing.T) {
	root := resolvedTempDir(t)
	var mu sync.Mutex
	points := []string{"/", filepath.Join(root, "excluded", ".snapshots")}
	mountPoints := func() ([]string, error) {
		mu.Lock()
		defer mu.Unlock()
		return points, nil
	}
	recorded, err := recordRoot(root, mountPoints)
	if err != nil {
		t.Fatal(err)
	}
	walked := func(directory string) bool { return directory == root }
	if recorded.mountAppeared(walked) {
		t.Fatal("the mounts listed when the layout was recorded appeared")
	}
	// A snapshot ZFS mounts beneath its hidden .zfs directory is never walked.
	mu.Lock()
	points = append(points, filepath.Join(root, ".zfs", "snapshot", "daily"))
	mu.Unlock()
	if recorded.mountAppeared(walked) {
		t.Fatal("a mount beneath a directory the walk never reaches appeared")
	}
	mu.Lock()
	points = append(points, filepath.Join(root, "disk2"))
	mu.Unlock()
	if !recorded.mountAppeared(walked) {
		t.Fatal("a disk mounted in the root did not appear")
	}
}

// A mount table that could not be read when the layout was recorded gives
// nothing to compare with, so nothing counts as appeared.
func TestLayoutIgnoresMountsWhenTheTableWasUnreadable(t *testing.T) {
	root := resolvedTempDir(t)
	failing := true
	mountPoints := func() ([]string, error) {
		if failing {
			return nil, errors.New("mount table unreadable")
		}
		return []string{root}, nil
	}
	recorded, err := recordRoot(root, mountPoints)
	if err != nil {
		t.Fatal(err)
	}
	failing = false
	if recorded.mountAppeared(func(string) bool { return true }) {
		t.Fatal("a root mounted all along appeared")
	}
}
