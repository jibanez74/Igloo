package movie

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) cleanupMissingMovie(ctx context.Context, scan *movieScanContext, reconciliation *scanner.Reconciliation) (int, error) {
	return reconciliation.DeleteUnseen(ctx, func(file scanner.CatalogFile) (bool, error) {
		return s.deleteMissingMovie(ctx, scan, reconciliation, file)
	})
}

func (s *Scanner) deleteMissingMovie(ctx context.Context, scan *movieScanContext, reconciliation *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
	var roomIDs []int64
	deleted, err := reconciliation.DeleteConfirmed(ctx, s.tx, file, func(qtx *database.Queries) (bool, error) {
		var err error
		roomIDs, err = qtx.ListWatchRoomIDsByMovieID(ctx, file.ID)
		if err != nil {
			return false, err
		}
		rows, err := qtx.DeleteMissingMovie(ctx, database.DeleteMissingMovieParams{ID: file.ID, FilePath: file.Path})
		if err != nil {
			return false, err
		}
		return rows > 0, nil
	}, func() {
		s.invalidateDeletedWatchRooms(roomIDs)
		s.invalidateCommittedMovie(file.ID)
	})
	if err != nil || !deleted {
		return false, err
	}
	scan.deleteEntry(filepath.Clean(file.Path))
	return true, nil
}
