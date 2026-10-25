package movie

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

func (s *Scanner) Start() scanner.StartResult {
	return s.launcher.Launch(s.currentMoviesDirectory(), s.beginReport(scanner.ScopeLibrary), func(directory string) {
		s.runMovieScan(directory, nil)
	})
}

// StartChanges applies what the library watcher saw change. A catch-up walks
// the whole library, so it starts like a library scan.
func (s *Scanner) StartChanges(changes scanner.Changes) scanner.StartResult {
	run := func(directory string) { s.runMovieScan(directory, &changes) }
	if changes.CatchUp {
		return s.launcher.Launch(s.currentMoviesDirectory(), s.beginReport(scanner.ScopeLibrary), run)
	}
	return s.launcher.LaunchChanges(s.currentMoviesDirectory(), s.beginReport(scanner.ScopeChanges), run)
}

func (s *Scanner) loadMovieScanIndex(ctx context.Context) (map[string]movieScanEntry, []scanner.CatalogFile, error) {
	rows, err := s.queries.GetMovieScanIndex(ctx)
	if err != nil {
		return nil, nil, err
	}
	return movieScanIndex(rows)
}

// loadMovieScanIndexUnder reads only the catalog rows at or beneath paths,
// for a run of changes.
func (s *Scanner) loadMovieScanIndexUnder(ctx context.Context, paths []string) (map[string]movieScanEntry, []scanner.CatalogFile, error) {
	rows := make([]database.GetMovieScanIndexRow, 0)
	for _, path := range paths {
		lower, upper := scanner.ChildBounds(path)
		under, err := s.queries.GetMovieScanIndexUnder(ctx, database.GetMovieScanIndexUnderParams{Path: path, Lower: lower, Upper: upper})
		if err != nil {
			return nil, nil, err
		}
		for _, row := range under {
			rows = append(rows, database.GetMovieScanIndexRow(row))
		}
	}
	return movieScanIndex(rows)
}

func movieScanIndex(rows []database.GetMovieScanIndexRow) (map[string]movieScanEntry, []scanner.CatalogFile, error) {
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
