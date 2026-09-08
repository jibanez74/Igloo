package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func TestReconciliationBoundariesAndSeenPaths(t *testing.T) {
	root := t.TempDir()
	files := []CatalogFile{
		{1, filepath.Join(root, "gone.mkv")},
		{2, root + "/nested/../seen.mkv"},
		{3, root + "-other/gone.mkv"},
		{4, filepath.Join(root, "..", "outside.mkv")},
		{5, root},
	}
	r, err := NewReconciliation(root+"/.", files)
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
			r, err := NewReconciliation(root, []CatalogFile{file})
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
	_, err := NewReconciliation(filepath.Join(root, "missing"), nil)
	if err == nil {
		t.Fatal("accepted absent root")
	}
	path := filepath.Join(root, "file")
	err = os.WriteFile(path, nil, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewReconciliation(path, nil)
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
			r, err := NewReconciliation(root, []CatalogFile{file})
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
	err := os.WriteFile(path, []byte("existing"), 0600)
	if err != nil {
		t.Fatal(err)
	}
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
		r, err := NewReconciliation(root, []CatalogFile{file})
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
