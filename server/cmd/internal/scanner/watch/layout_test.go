package watch

import (
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
	present, changed := recorded.check()
	if !present || changed {
		t.Fatalf("unchanged layout: present=%v changed=%v", present, changed)
	}
	mu.Lock()
	points = append(points, filepath.Join(root, "disk2"))
	mu.Unlock()
	present, changed = recorded.check()
	if !present || !changed {
		t.Fatalf("after disk2 mounted: present=%v changed=%v", present, changed)
	}
}
