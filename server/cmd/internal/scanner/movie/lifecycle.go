package movie

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/scanner"
)

func (s *Scanner) Start() StartResult {
	directory := s.currentMoviesDirectory()
	result := StartResult{Directory: directory.String}
	if !directory.Valid || directory.String == "" {
		result.Status = StartNotConfigured
		return result
	}

	if !s.guard.TryBegin() {
		result.Status = StartAlreadyRunning
		return result
	}

	// The run is published before the goroutine starts so a status poll right
	// after the request already sees it.
	s.beginReport()

	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		defer s.guard.Finish()
		s.runMovieScan(directory.String)
	}()
	result.Status = StartStarted
	return result
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
