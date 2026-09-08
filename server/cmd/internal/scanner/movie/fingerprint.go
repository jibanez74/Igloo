package movie

import (
	"context"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func storeMovieFingerprint(ctx context.Context, q *database.Queries, path string, f scanner.FileFingerprint) error {
	_, err := q.UpsertMovieFileFingerprint(ctx, database.UpsertMovieFileFingerprintParams{
		FilePath: path, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS,
		Device: f.Device, Inode: f.Inode, Sha256: f.SHA256[:],
	})
	return err
}

func (s *Scanner) persistFingerprint(ctx context.Context, scan *movieScanContext, path string, inspection *scanner.FileInspection) error {
	s.scannerDBMu.Lock()
	defer s.scannerDBMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = storeMovieFingerprint(ctx, s.queries.WithTx(tx), path, inspection.Fingerprint)
	if err != nil {
		return err
	}
	err = inspection.Validate(ctx)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	scan.movieIndex[path] = inspection.Fingerprint
	return nil
}
