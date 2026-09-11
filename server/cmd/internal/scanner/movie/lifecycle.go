package movie

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/scanner"
)

func (s *Scanner) Start() scanner.StartResult {
	return s.launcher.Launch(s.currentMoviesDirectory(), s.beginReport, s.runMovieScan)
}

func (s *Scanner) loadMovieScanIndex(ctx context.Context) (map[string]movieScanEntry, []scanner.CatalogFile, error) {
	rows, err := s.queries.GetMovieScanIndex(ctx)
	if err != nil {
		return nil, nil, err
	}
	index := make(map[string]movieScanEntry, len(rows))
	files := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, scanner.CatalogFile{ID: row.ID, Path: row.FilePath})
		index[filepath.Clean(row.FilePath)] = movieScanEntry{
			FileFingerprint: scanner.StoredFingerprint(row.Size, row.MtimeNs, row.CtimeNs, row.Device, row.Inode),
			ID:              row.ID, FilePath: row.FilePath, TmdbID: row.TmdbID, PendingRetry: row.PendingRetry,
			RetryAttempts: row.RetryAttempts.Int64, LastAttemptAt: row.LastAttemptAt, HasFingerprint: row.MtimeNs.Valid,
		}
	}
	return index, files, nil
}
