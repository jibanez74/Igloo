package movie

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) Start() scanner.StartResult {
	return s.launcher.Start(s.currentMoviesDirectory, nil, s.beginReport, s.runMovieScan)
}

// StartChanges applies what the library watcher saw change.
func (s *Scanner) StartChanges(changes scanner.Changes) scanner.StartResult {
	return s.launcher.Start(s.currentMoviesDirectory, &changes, s.beginReport, s.runMovieScan)
}

// loadMovieScanIndex reads the catalog rows the run covers: all of them, or
// for a run of changes only those at or beneath paths.
func (s *Scanner) loadMovieScanIndex(ctx context.Context, changes *scanner.Changes, paths []string) (map[string]movieScanEntry, []scanner.CatalogFile, error) {
	var rows []database.GetMovieScanIndexRow
	var err error
	if changes.Scoped() {
		rows, err = scanner.LoadUnder(ctx, paths, func(ctx context.Context, path, lower, upper string) ([]database.GetMovieScanIndexUnderRow, error) {
			return s.queries.GetMovieScanIndexUnder(ctx, database.GetMovieScanIndexUnderParams{Path: path, Lower: lower, Upper: upper})
		}, func(row database.GetMovieScanIndexUnderRow) database.GetMovieScanIndexRow {
			return database.GetMovieScanIndexRow(row)
		})
	} else {
		rows, err = s.queries.GetMovieScanIndex(ctx)
	}
	if err != nil {
		return nil, nil, err
	}
	return movieScanIndex(rows)
}

func movieScanIndex(rows []database.GetMovieScanIndexRow) (map[string]movieScanEntry, []scanner.CatalogFile, error) {
	index := make(map[string]movieScanEntry, len(rows))
	files := make([]scanner.CatalogFile, 0, len(rows))
	for _, row := range rows {
		files = append(files, scanner.CatalogFile{ID: row.ID, Path: row.FilePath, Device: row.Device.String})
		index[filepath.Clean(row.FilePath)] = movieScanEntry{
			FileFingerprint: scanner.StoredFingerprint(row.Size, row.MtimeNs, row.CtimeNs, row.Device, row.Inode),
			ID:              row.ID, FilePath: row.FilePath, TmdbID: row.TmdbID, PendingRetry: row.PendingRetry,
			RetryAttempts: row.RetryAttempts.Int64, LastAttemptAt: row.LastAttemptAt, HasFingerprint: row.MtimeNs.Valid,
		}
	}
	return index, files, nil
}
