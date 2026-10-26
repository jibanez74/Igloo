package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"igloo/cmd/internal/database"
)

// CatalogFile retains the original database identity even when paths need
// cleaning. Device is the filesystem its stored fingerprint recorded; empty
// without one.
type CatalogFile struct {
	ID     int64
	Path   string
	Device string
}

// Reconciliation owns filesystem decisions for one completed library walk.
// Call MarkSeen before processing and ConfirmMissing again immediately before commit.
type Reconciliation struct {
	root       string
	identity   os.FileInfo
	candidates []CatalogFile
	seen       map[string]bool
	stat       func(string) (os.FileInfo, error)
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
	r := &Reconciliation{root: root, identity: identity, seen: make(map[string]bool), stat: os.Stat}
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

// Guard keeps a catch-up from deleting files that look unmounted rather than
// deleted: a filesystem that is not mounted, such as a disk that came up late
// after a power cut, leaves an empty, readable mount point. A walk that found
// no file at all keeps every file. Otherwise a missing file is kept when the
// walk found nothing of the filesystem its fingerprint recorded: it is not the
// root's, and no cataloged file the walk saw recorded it either, which also
// holds across a share mounted again under another device number. Guard
// returns how many files it kept and the issue to report, empty when it kept
// none. A library scan an admin requests deletes them, for a disk removed for
// good.
func (r *Reconciliation) Guard(changes *Changes) (int, string) {
	catchUp := changes != nil && changes.CatchUp
	if !catchUp || len(r.candidates) == 0 {
		return 0, ""
	}
	if len(r.seen) == 0 {
		kept := len(r.candidates)
		r.candidates = nil
		return kept, ReasonCleanupSkippedEmpty
	}
	present := map[string]bool{FingerprintOf(r.identity).Device: true}
	for _, file := range r.candidates {
		if r.saw(file) {
			present[file.Device] = true
		}
	}
	remaining := make([]CatalogFile, 0, len(r.candidates))
	for _, file := range r.candidates {
		unmounted := file.Device != "" && !present[file.Device]
		if !unmounted {
			remaining = append(remaining, file)
		}
	}
	kept := len(r.candidates) - len(remaining)
	r.candidates = remaining
	if kept == 0 {
		return 0, ""
	}
	return kept, ReasonCleanupKeptUnmounted
}

func (r *Reconciliation) MarkSeen(path string) {
	absolute, err := filepath.Abs(path)
	if err == nil {
		r.seen[absolute] = true
	}
}

func (r *Reconciliation) saw(file CatalogFile) bool {
	absolute, err := filepath.Abs(file.Path)
	return err == nil && r.seen[absolute]
}

func (r *Reconciliation) Unseen() []CatalogFile {
	files := make([]CatalogFile, 0)
	for _, file := range r.candidates {
		if !r.saw(file) {
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

// errDeletionSkipped aborts a DeleteConfirmed transaction without reporting an
// error: the row was already gone, or the file reappeared before commit.
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
	missing, err := r.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	err = tx.Run(ctx, func(qtx *database.Queries) error {
		deleted, err := del(qtx)
		if err != nil {
			return err
		}
		if !deleted {
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
