package movie

import (
	"context"
	"database/sql"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func storeMovieFingerprint(ctx context.Context, q *database.Queries, path string, f scanner.FileFingerprint) error {
	rows, err := q.UpsertMovieFileFingerprint(ctx, database.UpsertMovieFileFingerprintParams{
		FilePath: path, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS,
		Device: f.Device, Inode: f.Inode,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
