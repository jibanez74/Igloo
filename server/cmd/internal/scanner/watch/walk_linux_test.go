package watch

import (
	"os"
	"testing"
)

// The first directory of a subtree walk has no visited parent; a mount there
// must still be recognized. /proc is its own filesystem on Linux.
func TestWalkLibraryDirsRecognizesAMountAtTheSubtreeRoot(t *testing.T) {
	proc, err := os.Stat("/proc")
	if err != nil {
		t.Skip("no /proc")
	}
	rootInfo, err := os.Stat("/")
	if err != nil || identityOf(rootInfo).device == identityOf(proc).device {
		t.Skip("/proc is not a separate mount here")
	}
	mounts := map[string]bool{}
	only := func(_, path string) bool { return path == "/proc" }
	err = walkLibraryDirs("/", "/proc", only, nil, nil, func(path string, _ os.FileInfo, mount bool) error {
		mounts[path] = mount
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !mounts["/proc"] || len(mounts) != 1 {
		t.Fatalf("visited %v, want /proc as a mount", mounts)
	}
}
