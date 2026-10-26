package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner/scannertest"
)

// cataloged returns a catalog file for path with the fingerprint its file has
// now, as a scan recorded it.
func cataloged(t *testing.T, id int64, path string) CatalogFile {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return CatalogFile{ID: id, Path: path, Fingerprint: FingerprintOf(info)}
}

func TestMovedFromPairsARenamedFileWithItsCatalogRow(t *testing.T) {
	root, tx := guardFixture(t)
	old := scannertest.WriteFile(t, filepath.Join(root, "A", "old.mkv"), "old")
	shared := scannertest.WriteFile(t, filepath.Join(root, "B", "shared.mkv"), "shared")
	files := []CatalogFile{
		cataloged(t, 1, old),
		cataloged(t, 2, shared),
		// Two rows with one fingerprint match neither.
		cataloged(t, 3, shared),
		{ID: 4, Path: filepath.Join(root, "unrecorded.mkv")},
	}
	files[2].Path = filepath.Join(root, "B", "twin.mkv")
	renamed := filepath.Join(root, "A", "new.mkv")
	err := os.Rename(old, renamed)
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(root, "C", "shared.mkv")
	err = os.MkdirAll(filepath.Dir(moved), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(shared, moved)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewReconciliation(context.Background(), root, files)
	if err != nil {
		t.Fatal(err)
	}

	from, found := r.MovedFrom(renamed)
	if !found || from.ID != 1 {
		t.Fatalf("MovedFrom(renamed) = %+v, %v", from, found)
	}
	from, found = r.MovedFrom(moved)
	if found {
		t.Fatalf("a fingerprint two rows share matched %+v", from)
	}
	from, found = r.MovedFrom(filepath.Join(root, "missing.mkv"))
	if found {
		t.Fatalf("a file that cannot be stat'ed matched %+v", from)
	}

	rekeyed := false
	committed, err := r.MoveConfirmed(context.Background(), tx, files[0], func(*database.Queries) (bool, error) {
		rekeyed = true
		return true, nil
	}, nil)
	if err != nil || !committed || !rekeyed {
		t.Fatalf("MoveConfirmed = %v, %v (re-keyed %v)", committed, err, rekeyed)
	}
	for _, file := range r.Unseen() {
		if file.ID == 1 {
			t.Fatal("a moved file is still up for cleanup")
		}
	}
	_, found = r.MovedFrom(renamed)
	if found {
		t.Fatal("a moved file matched again")
	}
}

// A file still at its old path was copied or linked, not moved.
func TestMoveConfirmedLeavesAFileStillAtItsOldPath(t *testing.T) {
	root, tx := guardFixture(t)
	old := scannertest.WriteFile(t, filepath.Join(root, "old.mkv"), "old")
	link := filepath.Join(root, "link.mkv")
	err := os.Link(old, link)
	if err != nil {
		t.Fatal(err)
	}
	file := cataloged(t, 1, old)
	r, err := NewReconciliation(context.Background(), root, []CatalogFile{file})
	if err != nil {
		t.Fatal(err)
	}
	from, found := r.MovedFrom(link)
	if !found {
		t.Fatal("the link was not paired with its catalog row")
	}
	committed, err := r.MoveConfirmed(context.Background(), tx, from, func(*database.Queries) (bool, error) {
		t.Fatal("a file still at its old path was re-keyed")
		return true, nil
	}, nil)
	if err != nil || committed {
		t.Fatalf("MoveConfirmed = %v, %v", committed, err)
	}
}
