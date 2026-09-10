package music

import (
	"context"
	"database/sql"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func storeTrackFingerprint(ctx context.Context, q *database.Queries, path string, f scanner.FileFingerprint) error {
	rows, err := q.UpsertTrackFileFingerprint(ctx, database.UpsertTrackFileFingerprintParams{
		FilePath: path, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS,
		Device: f.Device, Inode: f.Inode, Sha256: f.SHA256[:],
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Scanner) persistFingerprint(ctx context.Context, scan *musicScanContext, path string, inspection *scanner.FileInspection) error {
	s.scannerDBMu.Lock()
	defer s.scannerDBMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	err = storeTrackFingerprint(ctx, s.queries.WithTx(tx), path, inspection.Fingerprint)
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
	scan.trackIndex[path] = inspection.Fingerprint
	return nil
}
