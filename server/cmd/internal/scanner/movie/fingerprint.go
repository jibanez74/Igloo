package movie

import (
	"context"
	"time"

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
	qtx := s.queries.WithTx(tx)
	current, err := qtx.GetMovieByPath(ctx, path)
	if err != nil {
		return err
	}
	baseline := scan.movieIndex[path]
	if current.ID != baseline.ID || current.FilePath != baseline.FilePath {
		return nil
	}
	err = storeMovieFingerprint(ctx, qtx, path, inspection.Fingerprint)
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
	baseline.FileFingerprint = inspection.Fingerprint
	scan.movieIndex[path] = baseline
	return nil
}

// Unchanged inspections have no open descriptor. Recheck their filesystem
// metadata using the detector's stat-only (quiet-period) path, without hashing
// under the database mutex or changing the shared filesystem lifecycle.
func validateMovieInspection(ctx context.Context, path string, inspection *scanner.FileInspection) error {
	if inspection.Outcome != scanner.FileUnchanged {
		return inspection.Validate(ctx)
	}
	current, err := scanner.InspectFile(ctx, path, nil, func() time.Time { return time.Time{} })
	if err != nil {
		return err
	}
	defer current.Close()
	observed := current.Fingerprint
	observed.SHA256 = inspection.Fingerprint.SHA256
	if observed != inspection.Fingerprint {
		return &scanner.FileDeferral{Reason: scanner.FileChanged, Fingerprint: observed}
	}
	return nil
}
