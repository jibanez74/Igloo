package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"igloo/cmd/internal/database"
)

// CatalogFile retains the original database identity even when paths need cleaning.
type CatalogFile struct {
	ID   int64
	Path string
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
		contained := relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
		if contained {
			r.candidates = append(r.candidates, file)
		}
	}
	return r, nil
}

func (r *Reconciliation) MarkSeen(path string) {
	absolute, err := filepath.Abs(path)
	if err == nil {
		r.seen[absolute] = true
	}
}

func (r *Reconciliation) Unseen() []CatalogFile {
	files := make([]CatalogFile, 0)
	for _, file := range r.candidates {
		absolute, err := filepath.Abs(file.Path)
		if err == nil && !r.seen[absolute] {
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
