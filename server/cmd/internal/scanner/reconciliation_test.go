package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"syscall"
	"testing"

	"igloo/cmd/internal/scanner/scannertest"
)

func TestReconciliationBoundariesAndSeenPaths(t *testing.T) {
	root := t.TempDir()
	files := []CatalogFile{
		{ID: 1, Path: filepath.Join(root, "gone.mkv")},
		{ID: 2, Path: root + "/nested/../seen.mkv"},
		{ID: 3, Path: root + "-other/gone.mkv"},
		{ID: 4, Path: filepath.Join(root, "..", "outside.mkv")},
		{ID: 5, Path: root},
	}
	r, err := NewReconciliation(context.Background(), root+"/.", files)
	if err != nil {
		t.Fatal(err)
	}
	r.MarkSeen(filepath.Join(root, "seen.mkv"))
	if !reflect.DeepEqual(r.Unseen(), files[:1]) {
		t.Fatalf("unseen = %+v", r.Unseen())
	}
}

func TestReconciliationConfirmsOnlyNonexistence(t *testing.T) {
	for _, name := range []string{"missing", "removed directory", "broken symlink", "regular", "directory", "fifo", "symlink loop", "permission", "io", "not directory"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "file.mkv")
			file := CatalogFile{ID: 1, Path: path}
			r, err := NewReconciliation(context.Background(), root, []CatalogFile{file})
			if err != nil {
				t.Fatal(err)
			}
			wantMissing := false
			switch name {
			case "missing":
				wantMissing = true
			case "removed directory":
				file.Path = filepath.Join(root, "removed", "file.mkv")
				wantMissing = true
			case "broken symlink":
				err = os.Symlink(filepath.Join(root, "absent"), path)
				wantMissing = true
			case "regular":
				err = os.WriteFile(path, []byte("media"), 0600)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "fifo":
				err = syscall.Mkfifo(path, 0600)
			case "symlink loop":
				err = os.Symlink(path, path)
			case "permission", "io", "not directory":
				failure := syscall.EACCES
				if name == "io" {
					failure = syscall.EIO
				}
				if name == "not directory" {
					failure = syscall.ENOTDIR
				}
				r.stat = func(p string) (os.FileInfo, error) {
					if p == path {
						return nil, &os.PathError{Op: "stat", Path: p, Err: failure}
					}
					return os.Stat(p)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			missing, err := r.ConfirmMissing(context.Background(), file)
			if missing != wantMissing {
				t.Fatalf("missing=%v error=%v", missing, err)
			}
		})
	}
}

func TestReconciliationRejectsUnavailableRoot(t *testing.T) {
	root := t.TempDir()
	_, err := NewReconciliation(context.Background(), filepath.Join(root, "missing"), nil)
	if err == nil {
		t.Fatal("accepted absent root")
	}
	path := filepath.Join(root, "file")
	err = os.WriteFile(path, nil, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewReconciliation(context.Background(), path, nil)
	if err == nil {
		t.Fatal("accepted file root")
	}
}

func TestReconciliationRevalidatesRootAndAbsence(t *testing.T) {
	for _, name := range []string{"removed root", "replaced root", "unreadable root", "canceled", "reappeared", "root lost during stat", "fifo root"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "library")
			err := os.Mkdir(root, 0700)
			if err != nil {
				t.Fatal(err)
			}
			file := CatalogFile{ID: 1, Path: filepath.Join(root, "missing.mkv")}
			r, err := NewReconciliation(context.Background(), root, []CatalogFile{file})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			missing, err := r.ConfirmMissing(ctx, file)
			if !missing || err != nil {
				t.Fatalf("initial absence: %v %v", missing, err)
			}
			switch name {
			case "removed root":
				err = os.Remove(root)
			case "replaced root":
				err = os.Rename(root, root+"-old")
				if err == nil {
					err = os.Mkdir(root, 0700)
				}
			case "fifo root":
				err = os.Rename(root, root+"-old")
				if err == nil {
					err = syscall.Mkfifo(root, 0600)
				}
			case "unreadable root":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses directory permissions")
				}
				err = os.Chmod(root, 0000)
				t.Cleanup(func() { _ = os.Chmod(root, 0700) })
			case "canceled":
				cancel()
			case "reappeared":
				err = os.WriteFile(file.Path, []byte("back"), 0600)
			case "root lost during stat":
				r.stat = func(path string) (os.FileInfo, error) {
					if path == file.Path {
						removeErr := os.Remove(root)
						if removeErr != nil {
							t.Fatal(removeErr)
						}
					}
					return os.Stat(path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			missing, err = r.ConfirmMissing(ctx, file)
			if missing {
				t.Fatal("unsafe deletion permitted")
			}
			if name == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestReconciliationUsesCleanedAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file.mkv")
	scannertest.WriteFile(t, path, "existing")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range []string{relative, root + "/absent/../file.mkv"} {
		file := CatalogFile{ID: 1, Path: stored}
		r, err := NewReconciliation(context.Background(), root, []CatalogFile{file})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Unseen()) != 1 {
			t.Fatal("cleaned candidate was excluded")
		}
		missing, err := r.ConfirmMissing(context.Background(), file)
		if missing || err != nil {
			t.Fatalf("existing cleaned path: %v %v", missing, err)
		}
		r.MarkSeen(path)
		if len(r.Unseen()) != 0 {
			t.Fatal("absolute seen path did not protect stored path")
		}
	}
}

// guardFixture is a resolved library root with a database to record its
// mount points in.
func guardFixture(t *testing.T) (string, TxRunner) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, queries := scannertest.OpenDB(t, ":memory:")
	return root, TxRunner{DB: db, Mu: &sync.Mutex{}, Queries: queries}
}

// mountTable answers MountPoints with the given mount points.
func mountTable(points ...string) func() ([]string, error) {
	return func() ([]string, error) { return points, nil }
}

func TestReconciliationGuardKeepsEverythingAfterACatchUpThatFoundNothing(t *testing.T) {
	root, tx := guardFixture(t)
	files := []CatalogFile{{ID: 1, Path: filepath.Join(root, "A", "a.mkv")}}
	for _, tc := range []struct {
		changes *Changes
		kept    int
	}{
		{&Changes{CatchUp: true}, 1},
		{nil, 0},
		{&Changes{CatchUp: true, Manual: true}, 0},
		{&Changes{Paths: []string{"A"}}, 0},
	} {
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		kept, err := r.Guard(context.Background(), tx, tc.changes, mountTable())
		if err != nil {
			t.Fatal(err)
		}
		if kept.Files != tc.kept || len(r.Unseen()) != 1-tc.kept {
			t.Fatalf("%+v: kept %+v, unseen %v", tc.changes, kept, r.Unseen())
		}
		if tc.kept > 0 && (kept.Reason != ReasonCleanupSkippedEmpty || kept.MountPoints != nil) {
			t.Fatalf("an empty walk keeps the whole library: %+v", kept)
		}
	}
}

// A disk mounted inside the library that has not come up, as after a power
// cut, leaves an empty mount point. Its missing files are kept by every
// catch-up until it is back, whatever device numbers the disks get, while a
// file deleted from a mounted disk still goes; a library scan an admin asks
// for deletes them and forgets the mount point.
func TestReconciliationGuardKeepsFilesBeneathAMountPointThatIsNotMounted(t *testing.T) {
	root, tx := guardFixture(t)
	disk1, disk2 := filepath.Join(root, "disk1"), filepath.Join(root, "disk2")
	files := []CatalogFile{
		{ID: 1, Path: filepath.Join(disk1, "seen.mkv")},
		{ID: 2, Path: filepath.Join(disk1, "gone.mkv")},
		{ID: 3, Path: filepath.Join(disk2, "a.mkv")},
		{ID: 4, Path: filepath.Join(disk2, "b.mkv")},
		{ID: 5, Path: filepath.Join(root, "gone.mkv")},
	}
	// guard runs the guard over a walk that saw files[0] and the files with
	// the extra ids.
	guard := func(changes *Changes, mounts func() ([]string, error), extra ...int64) (Kept, []int64) {
		t.Helper()
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		r.MarkSeen(files[0].Path)
		for _, file := range files {
			if slices.Contains(extra, file.ID) {
				r.MarkSeen(file.Path)
			}
		}
		kept, err := r.Guard(context.Background(), tx, changes, mounts)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, file := range r.Unseen() {
			ids = append(ids, file.ID)
		}
		return kept, ids
	}
	recorded := func() []string {
		t.Helper()
		points, err := tx.Queries.ListLibraryMountPoints(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		return points
	}

	// A snapshot ZFS mounted beneath its hidden .zfs directory is never
	// walked, so it is not recorded.
	snapshot := filepath.Join(root, ".zfs", "snapshot", "daily")
	kept, _ := guard(nil, mountTable(disk1, disk2, snapshot), 3, 4)
	if kept.Files != 0 || !slices.Equal(recorded(), []string{"disk1", "disk2"}) {
		t.Fatalf("a library scan with both disks mounted: kept %+v, recorded %q", kept, recorded())
	}
	for range 2 {
		kept, ids := guard(&Changes{CatchUp: true}, mountTable(disk1))
		if kept.Files != 2 || kept.Reason != ReasonCleanupKeptUnmounted || !slices.Equal(kept.MountPoints, []string{disk2}) {
			t.Fatalf("catch-up with disk2 missing kept %+v", kept)
		}
		if !slices.Equal(ids, []int64{2, 5}) {
			t.Fatalf("cleanup candidates %v, want the files deleted from mounted filesystems", ids)
		}
		if !slices.Equal(recorded(), []string{"disk1", "disk2"}) {
			t.Fatalf("a catch-up forgot a mount point: %q", recorded())
		}
	}
	kept, ids := guard(&Changes{CatchUp: true, Manual: true}, mountTable(disk1))
	if kept.Files != 0 || !slices.Equal(ids, []int64{2, 3, 4, 5}) || !slices.Equal(recorded(), []string{"disk1"}) {
		t.Fatalf("an admin's scan: kept %+v, candidates %v, recorded %q", kept, ids, recorded())
	}
	kept, _ = guard(&Changes{CatchUp: true}, mountTable(disk1))
	if kept.Files != 0 {
		t.Fatalf("a forgotten mount point still kept files: %+v", kept)
	}

	// A run of changes keeps nothing, but records a disk that mounted since,
	// whose first files it may import.
	disk3 := filepath.Join(root, "disk3")
	files = append(files, CatalogFile{ID: 6, Path: filepath.Join(disk3, "c.mkv")})
	kept, _ = guard(&Changes{Paths: []string{"disk3"}}, mountTable(disk1, disk3), 6)
	if kept.Files != 0 || !slices.Equal(recorded(), []string{"disk1", "disk3"}) {
		t.Fatalf("a run of changes: kept %+v, recorded %q", kept, recorded())
	}
	kept, _ = guard(&Changes{CatchUp: true}, mountTable(disk1))
	if kept.Files != 1 || !slices.Equal(kept.MountPoints, []string{disk3}) {
		t.Fatalf("a disk a run of changes recorded was not kept: %+v", kept)
	}
}

// A library whose root is itself a mount point keeps every missing file
// while it is not mounted, even when something left in the empty mount point
// is found.
func TestReconciliationGuardKeepsTheFilesOfARootThatIsNotMounted(t *testing.T) {
	root, tx := guardFixture(t)
	files := []CatalogFile{{ID: 1, Path: filepath.Join(root, "stray.mkv")}, {ID: 2, Path: filepath.Join(root, "A", "a.mkv")}}
	for _, tc := range []struct {
		mounts func() ([]string, error)
		kept   int
	}{{mountTable(root), 0}, {mountTable(), 1}} {
		r, err := NewReconciliation(context.Background(), root, files)
		if err != nil {
			t.Fatal(err)
		}
		r.MarkSeen(files[0].Path)
		kept, err := r.Guard(context.Background(), tx, &Changes{CatchUp: true}, tc.mounts)
		if err != nil {
			t.Fatal(err)
		}
		if kept.Files != tc.kept {
			t.Fatalf("kept %+v, want %d", kept, tc.kept)
		}
	}
}

// A library configured through a symlink records and checks its mount points
// under the resolved root, the paths the mount table holds.
func TestReconciliationGuardResolvesASymlinkedRoot(t *testing.T) {
	resolved, tx := guardFixture(t)
	link := filepath.Join(t.TempDir(), "library")
	err := os.Symlink(resolved, link)
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(resolved, "disk")
	files := []CatalogFile{{ID: 1, Path: filepath.Join(link, "seen.mkv")}, {ID: 2, Path: filepath.Join(link, "disk", "a.mkv")}}
	for _, tc := range []struct {
		mounts func() ([]string, error)
		seen   []CatalogFile
		kept   int
	}{{mountTable(disk), files, 0}, {mountTable(), files[:1], 1}} {
		r, err := NewReconciliation(context.Background(), link, files)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range tc.seen {
			r.MarkSeen(file.Path)
		}
		kept, err := r.Guard(context.Background(), tx, &Changes{CatchUp: true}, tc.mounts)
		if err != nil {
			t.Fatal(err)
		}
		if kept.Files != tc.kept || len(r.Unseen()) != len(files)-len(tc.seen)-tc.kept {
			t.Fatalf("kept %+v, want %d; unseen %v", kept, tc.kept, r.Unseen())
		}
	}
}

// A mount table that cannot be read fails the guard before anything is
// recorded or deleted.
func TestReconciliationGuardFailsWithoutAMountTable(t *testing.T) {
	root, tx := guardFixture(t)
	files := []CatalogFile{{ID: 1, Path: filepath.Join(root, "seen.mkv")}, {ID: 2, Path: filepath.Join(root, "gone.mkv")}}
	r, err := NewReconciliation(context.Background(), root, files)
	if err != nil {
		t.Fatal(err)
	}
	r.MarkSeen(files[0].Path)
	unreadable := errors.New("mount table unreadable")
	_, err = r.Guard(context.Background(), tx, &Changes{CatchUp: true}, func() ([]string, error) { return nil, unreadable })
	if !errors.Is(err, unreadable) {
		t.Fatalf("Guard error = %v", err)
	}
}
