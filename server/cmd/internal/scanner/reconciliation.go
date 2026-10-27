package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"igloo/cmd/internal/database"
)

// CatalogFile retains the original database identity even when paths need
// cleaning, with the stored fingerprint of its file; zero without one.
type CatalogFile struct {
	ID          int64
	Path        string
	Fingerprint FileFingerprint
}

// Reconciliation owns filesystem decisions for one completed library walk.
// Call MarkSeen before processing and ConfirmMissing again immediately before commit.
type Reconciliation struct {
	root       string
	identity   os.FileInfo
	candidates []CatalogFile
	seen       map[string]bool
	stat       func(string) (os.FileInfo, error)
	// moves indexes the candidates by what a move keeps, built on first use;
	// moved holds the ids of those re-keyed to a new path, out of the cleanup.
	moves map[moveKey][]int
	moved map[int64]bool
}

func NewReconciliation(ctx context.Context, directory string, files []CatalogFile) (*Reconciliation, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	identity, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	r := &Reconciliation{root: root, identity: identity, seen: make(map[string]bool), stat: os.Stat, moved: make(map[int64]bool)}
	err = r.ValidateRoot(ctx)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		path, err := filepath.Abs(file.Path)
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		contained := relative != "." && !escapesRoot(relative)
		if contained {
			r.candidates = append(r.candidates, file)
		}
	}
	return r, nil
}

// Kept is what a cleanup guard held back.
type Kept struct {
	// Files counts the missing files kept.
	Files int
	// Reason is the issue to report for them.
	Reason string
	// MountPoints are the absolute paths of the recorded mount points, not
	// mounted now, that the kept files sit beneath; none when the walk found
	// no file at all.
	MountPoints []string
}

// Guard prepares a run's cleanup. A walk of the whole library the server
// started on its own, a catch-up, keeps files that look unmounted rather than
// deleted, since a filesystem that is not mounted leaves an empty, readable
// mount point: every file when the walk found none, and otherwise the missing
// files beneath a mount point an earlier run recorded that is not mounted
// now, such as a disk that came up late after a power cut. Mount points are
// recorded by path, since device numbers follow probe order and change across
// boots. Every run that reaches cleanup adds the mount points beneath the
// root it walked through to the record, a run of changes too, which may have
// imported a disk's first files, so a disk still missing stays protected; a
// library scan an admin asked for deletes every missing file and records only
// what it walked through.
func (r *Reconciliation) Guard(ctx context.Context, tx TxRunner, changes *Changes, mountPoints func() ([]string, error)) (Kept, error) {
	points, err := mountPoints()
	if err != nil {
		return Kept{}, err
	}
	resolved, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		return Kept{}, err
	}
	mounted := MountPointsBeneath(points, resolved)
	recorded, err := tx.Queries.ListLibraryMountPoints(ctx, r.root)
	if err != nil {
		return Kept{}, err
	}
	walked := r.walked(mounted)
	record := append(slices.Clone(recorded), walked...)
	manual := changes == nil || changes.Manual
	if manual {
		record = walked
	}
	kept := Kept{}
	guarded := changes.Guarded()
	if guarded {
		kept = r.keep(recorded, mounted)
	}
	slices.Sort(record)
	record = slices.Compact(record)
	unchanged := slices.Equal(record, recorded)
	if unchanged {
		return kept, nil
	}
	err = tx.Run(ctx, func(q *database.Queries) error {
		err := q.DeleteLibraryMountPoints(ctx, r.root)
		if err != nil {
			return err
		}
		for _, point := range record {
			err = q.InsertLibraryMountPoint(ctx, database.InsertLibraryMountPointParams{Root: r.root, MountPoint: point})
			if err != nil {
				return err
			}
		}
		return nil
	}, nil)
	return kept, err
}

// walked returns the mount points among points that the run walked through:
// the root, or one beneath which it saw a file. Mounts it never reaches, such
// as the snapshots ZFS mounts beneath its hidden .zfs directory, are left
// out of the record.
func (r *Reconciliation) walked(points []string) []string {
	var relatives []string
	for path := range r.seen {
		relative, err := filepath.Rel(r.root, path)
		if err == nil {
			relatives = append(relatives, relative)
		}
	}
	var walked []string
	for _, point := range points {
		reached := point == "."
		for _, relative := range relatives {
			reached = reached || PathWithin(point, relative)
		}
		if reached {
			walked = append(walked, point)
		}
	}
	return walked
}

// keep takes the files a catch-up keeps out of the cleanup.
func (r *Reconciliation) keep(recorded, mounted []string) Kept {
	if len(r.candidates) == 0 {
		return Kept{}
	}
	if len(r.seen) == 0 {
		kept := Kept{Files: len(r.candidates), Reason: ReasonCleanupSkippedEmpty}
		r.candidates = nil
		return kept
	}
	var unmounted []string
	for _, point := range recorded {
		stillMounted := slices.Contains(mounted, point)
		if !stillMounted {
			unmounted = append(unmounted, point)
		}
	}
	if len(unmounted) == 0 {
		return Kept{}
	}
	kept := Kept{Reason: ReasonCleanupKeptUnmounted}
	held := make(map[string]bool)
	remaining := make([]CatalogFile, 0, len(r.candidates))
	for _, file := range r.candidates {
		point := r.mountPointOf(file, unmounted)
		seen := r.saw(file)
		if point == "" || seen || r.moved[file.ID] {
			remaining = append(remaining, file)
			continue
		}
		kept.Files++
		if !held[point] {
			held[point] = true
			kept.MountPoints = append(kept.MountPoints, filepath.Join(r.root, point))
		}
	}
	r.candidates = remaining
	return kept
}

// mountPointOf returns the mount point among points, relative to the root,
// that file sits beneath, or "".
func (r *Reconciliation) mountPointOf(file CatalogFile, points []string) string {
	absolute, err := filepath.Abs(file.Path)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(r.root, absolute)
	if err != nil {
		return ""
	}
	for _, point := range points {
		beneath := point == "." || PathWithin(point, relative)
		if beneath {
			return point
		}
	}
	return ""
}

func (r *Reconciliation) MarkSeen(path string) {
	absolute, err := filepath.Abs(path)
	if err == nil {
		r.seen[absolute] = true
	}
}

// Seen reports a path the walk saw.
func (r *Reconciliation) Seen(path string) bool {
	absolute, err := filepath.Abs(path)
	return err == nil && r.seen[absolute]
}

func (r *Reconciliation) saw(file CatalogFile) bool {
	absolute, err := filepath.Abs(file.Path)
	return err == nil && r.seen[absolute]
}

func (r *Reconciliation) Unseen() []CatalogFile {
	files := make([]CatalogFile, 0)
	for _, file := range r.candidates {
		seen := r.saw(file)
		if !seen && !r.moved[file.ID] {
			files = append(files, file)
		}
	}
	return files
}

// ValidateRoot checks readability and identity, including an empty library.
func (r *Reconciliation) ValidateRoot(ctx context.Context) error {
	err := ctx.Err()
	if err != nil {
		return err
	}
	// O_DIRECTORY rejects a raced replacement with a FIFO without blocking.
	directory, err := os.OpenFile(r.root, os.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer directory.Close()
	info, err := directory.Stat()
	if err != nil {
		return err
	}
	same := info.IsDir() && os.SameFile(r.identity, info)
	if !same {
		return fmt.Errorf("library root changed: %s", r.root)
	}
	_, err = directory.Readdirnames(1)
	empty := errors.Is(err, io.EOF)
	if err != nil && !empty {
		return err
	}
	current, err := r.stat(r.root)
	if err != nil {
		return err
	}
	same = os.SameFile(r.identity, current)
	if !same {
		return fmt.Errorf("library root changed: %s", r.root)
	}
	return ctx.Err()
}

func (r *Reconciliation) ConfirmMissing(ctx context.Context, file CatalogFile) (bool, error) {
	err := r.ValidateRoot(ctx)
	if err != nil {
		return false, err
	}
	path, err := filepath.Abs(file.Path)
	if err != nil {
		return false, err
	}
	_, err = r.stat(path)
	missing := errors.Is(err, os.ErrNotExist)
	// An inaccessible path is not evidence of deletion. Other candidates can
	// still be reconciled, and music enrichment can proceed.
	if err != nil && !missing {
		return false, nil
	}
	err = r.ValidateRoot(ctx)
	return missing && err == nil, err
}

// errDeletionSkipped aborts a deletion or move transaction without reporting
// an error: the row no longer matched, or the file reappeared before commit.
var errDeletionSkipped = errors.New("deletion skipped")

// DeleteUnseen deletes every catalog file the walk did not see, through del,
// and returns how many deletions committed. The first error stops the loop.
func (r *Reconciliation) DeleteUnseen(ctx context.Context, del func(CatalogFile) (bool, error)) (int, error) {
	err := r.ValidateRoot(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, file := range r.Unseen() {
		committed, err := del(file)
		if err != nil {
			return deleted, err
		}
		if committed {
			deleted++
		}
	}
	return deleted, ctx.Err()
}

// DeleteConfirmed confirms the file is missing, runs del inside a transaction,
// confirms again immediately before commit, and reports whether the deletion
// committed. del returns false when the catalog row no longer matches;
// committed runs under the scanner mutex once the deletion is durable.
func (r *Reconciliation) DeleteConfirmed(
	ctx context.Context,
	tx TxRunner,
	file CatalogFile,
	del func(*database.Queries) (bool, error),
	committed func(),
) (bool, error) {
	return r.whileMissing(ctx, tx, file, del, committed)
}

// whileMissing runs change inside a transaction while file is confirmed
// missing, before it and again immediately before commit, and reports whether
// the change committed. change returns false when the catalog row no longer
// matches; committed runs under the scanner mutex once the change is durable.
func (r *Reconciliation) whileMissing(
	ctx context.Context,
	tx TxRunner,
	file CatalogFile,
	change func(*database.Queries) (bool, error),
	committed func(),
) (bool, error) {
	missing, err := r.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	err = tx.Run(ctx, func(qtx *database.Queries) error {
		changed, err := change(qtx)
		if err != nil {
			return err
		}
		if !changed {
			return errDeletionSkipped
		}
		missing, err := r.ConfirmMissing(ctx, file)
		if err != nil {
			return err
		}
		if !missing {
			return errDeletionSkipped
		}
		return nil
	}, committed)
	skipped := errors.Is(err, errDeletionSkipped)
	if skipped {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
