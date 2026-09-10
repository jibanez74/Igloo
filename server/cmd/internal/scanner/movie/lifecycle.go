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

	s.beginReport()

	// Add/Done are paired here so runMovieScan stays callable on its own.
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
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
			FileFingerprint: scanner.FileFingerprint{Size: row.Size, MtimeNS: row.MtimeNs.Int64, CtimeNS: row.CtimeNs.Int64,
				Device: row.Device.String, Inode: row.Inode.String},
			ID: row.ID, FilePath: row.FilePath, TmdbID: row.TmdbID, PendingRetry: row.PendingRetry, HasFingerprint: row.MtimeNs.Valid,
		}
	}
	return index, files, nil
}
