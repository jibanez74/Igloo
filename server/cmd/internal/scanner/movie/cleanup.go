package movie

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) cleanupMissingMovie(ctx context.Context, scan *movieScanContext, reconciliation *scanner.Reconciliation) (int, error) {
	err := reconciliation.ValidateRoot(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, file := range reconciliation.Unseen() {
		committed, err := s.deleteMissingMovie(ctx, scan, reconciliation, file)
		if err != nil {
			return deleted, err
		}
		if committed {
			deleted++
		}
	}
	return deleted, ctx.Err()
}

func (s *Scanner) deleteMissingMovie(ctx context.Context, scan *movieScanContext, reconciliation *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
	missing, err := reconciliation.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	s.scannerDBMu.Lock()
	defer s.scannerDBMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	qtx := s.queries.WithTx(tx)

	rows, err := qtx.DeleteMissingMovie(ctx, database.DeleteMissingMovieParams{ID: file.ID, FilePath: file.Path})
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, nil
	}

	missing, err = reconciliation.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	err = tx.Commit()
	if err != nil {
		return false, err
	}
	s.invalidateCommittedMovie(file.ID)
	delete(scan.movieIndex, filepath.Clean(file.Path))
	return true, nil
}
