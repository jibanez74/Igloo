package movie

import (
	"context"
	"database/sql"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

// applyMoves re-keys each movie the walk found under a path the catalog does
// not hold, when it is a cataloged file renamed or moved within the library,
// so it keeps its row: watch progress, playlists, watch rooms and its TMDB
// match. Processing then sees the file at its new path as changed metadata.
func (s *Scanner) applyMoves(ctx context.Context, scan *movieScanContext, reconciliation *scanner.Reconciliation, files []localFile) {
	for _, file := range files {
		path := file.file.Path
		_, cataloged := scan.movieIndex[path]
		if cataloged {
			continue
		}
		from, found := reconciliation.MovedFrom(path)
		if found {
			s.moveMovie(ctx, scan, reconciliation, from, path)
		}
	}
}

// moveMovie re-keys one movie. An unmatched movie whose new name searches
// differently is searched again at once rather than after its miss backoff,
// since a rename is how a wrong or missing match is fixed.
func (s *Scanner) moveMovie(ctx context.Context, scan *movieScanContext, reconciliation *scanner.Reconciliation, from scanner.CatalogFile, to string) {
	old := filepath.Clean(from.Path)
	entry := scan.movieIndex[old]
	oldTitle, oldYear := tmdbSearchTitle(old)
	newTitle, newYear := tmdbSearchTitle(to)
	searchAgain := !entry.TmdbID.Valid && (oldTitle != newTitle || oldYear != newYear)
	moved, err := reconciliation.MoveConfirmed(ctx, s.tx, from, func(q *database.Queries) (bool, error) {
		rows, err := q.MoveMovieFile(ctx, database.MoveMovieFileParams{NewPath: to, FileName: filepath.Base(to), ID: from.ID, FilePath: from.Path})
		if err != nil || rows == 0 {
			return false, err
		}
		if searchAgain {
			err = q.ClearMovieTmdbRetry(ctx, from.ID)
		}
		return true, err
	}, func() {
		s.invalidateCommittedMovie(from.ID)
	})
	if err != nil {
		s.logger.Warn("movie move failed; the file is imported as new", "from", from.Path, "to", to, "error", err)
		return
	}
	if !moved {
		return
	}
	scan.deleteEntry(old)
	entry.FilePath = to
	if searchAgain {
		entry.PendingRetry, entry.RetryAttempts, entry.LastAttemptAt = false, 0, sql.NullInt64{}
	}
	scan.setEntry(to, entry)
	s.logger.Info("movie file moved", "from", from.Path, "to", to)
}
