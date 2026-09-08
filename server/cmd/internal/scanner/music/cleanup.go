package music

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) cleanupMissingMusic(ctx context.Context, scan *musicScanContext, reconciliation *scanner.Reconciliation) (int, error) {
	err := reconciliation.ValidateRoot(ctx)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, file := range reconciliation.Unseen() {
		committed, err := s.deleteMissingTrack(ctx, scan, reconciliation, file)
		if err != nil {
			return deleted, err
		}
		if committed {
			deleted++
		}
	}
	return deleted, ctx.Err()
}

func (s *Scanner) deleteMissingTrack(ctx context.Context, scan *musicScanContext, reconciliation *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
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

	artists, err := qtx.MusicTrackAffectedArtists(ctx, file.Path)
	if err != nil {
		return false, err
	}
	album, err := qtx.MusicTrackAffectedAlbum(ctx, file.Path)
	notFound := errors.Is(err, sql.ErrNoRows)
	if notFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	rows, err := qtx.DeleteMissingTrack(ctx, database.DeleteMissingTrackParams{ID: file.ID, FilePath: file.Path})
	if err != nil {
		return false, err
	}
	if rows == 0 {
		return false, nil
	}

	for _, id := range artists {
		err = qtx.ReconcileMusicArtistSort(ctx, id)
		if err != nil {
			return false, err
		}
	}
	if album.Valid {
		err = reconcileAlbum(ctx, qtx, album.Int64)
		if err != nil {
			return false, err
		}
	}

	missing, err = reconciliation.ConfirmMissing(ctx, file)
	if err != nil || !missing {
		return false, err
	}
	err = tx.Commit()
	if err != nil {
		return false, err
	}
	s.invalidateCommittedTrack(file.ID)
	delete(scan.trackIndex, filepath.Clean(file.Path))
	return true, nil
}
