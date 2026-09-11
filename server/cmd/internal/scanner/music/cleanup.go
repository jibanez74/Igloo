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
	return reconciliation.DeleteUnseen(ctx, func(file scanner.CatalogFile) (bool, error) {
		return s.deleteMissingTrack(ctx, scan, reconciliation, file)
	})
}

func (s *Scanner) deleteMissingTrack(ctx context.Context, scan *musicScanContext, reconciliation *scanner.Reconciliation, file scanner.CatalogFile) (bool, error) {
	deleted, err := reconciliation.DeleteConfirmed(ctx, s.tx, file, func(qtx *database.Queries) (bool, error) {
		album, err := qtx.MusicTrackAffectedAlbum(ctx, file.Path)
		notFound := errors.Is(err, sql.ErrNoRows)
		if notFound {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		artists, err := qtx.MusicTrackAffectedArtists(ctx, file.Path)
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
		return true, nil
	}, func() {
		s.invalidateCommittedTrack(file.ID)
	})
	if err != nil || !deleted {
		return false, err
	}
	delete(scan.trackIndex, filepath.Clean(file.Path))
	return true, nil
}
