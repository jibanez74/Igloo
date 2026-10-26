package movie

import (
	"context"
	"database/sql"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

// applyMoves re-keys each movie the walk found under a path the catalog does
// not hold, when it is a cataloged file renamed or moved within the library,
// so it keeps its row: watch progress, playlists, watch rooms and its TMDB
// match. Processing then checks the file at its new path like any other: a
// renamed file's new ctime probes it again, which also catches an edit that
// kept its size and modification time, while a file in a renamed folder is
// unchanged.
func (s *Scanner) applyMoves(ctx context.Context, scan *movieScanContext, report *scanReport, reconciliation *scanner.Reconciliation, files []localFile) {
	moved := 0
	for _, file := range files {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		path := file.file.Path
		_, cataloged := scan.movieIndex[path]
		if cataloged {
			continue
		}
		move, found := reconciliation.MovedFrom(path)
		if !found {
			continue
		}
		committed := s.moveMovie(ctx, scan, report, reconciliation, move)
		if committed {
			moved++
		}
	}
	if moved > 0 {
		s.logger.Info("movie files moved", "files", moved)
	}
}

// moveMovie re-keys one movie. A renamed unmatched movie's title and year
// follow the new name, and when it searches differently it is searched again
// at once rather than after its miss backoff, since a rename is how a wrong
// or missing match is fixed.
func (s *Scanner) moveMovie(ctx context.Context, scan *movieScanContext, report *scanReport, reconciliation *scanner.Reconciliation, move scanner.Move) bool {
	old := filepath.Clean(move.From.Path)
	entry := scan.movieIndex[old]
	oldTitle, oldYear := tmdbSearchTitle(old)
	newTitle, newYear := tmdbSearchTitle(move.To)
	searchAgain := !entry.TmdbID.Valid && (oldTitle != newTitle || oldYear != newYear)
	titleYear := movieTitleYear(move.To)
	moved, err := reconciliation.MoveConfirmed(ctx, s.tx, move, &report.Report, func(q *database.Queries) (bool, error) {
		rows, err := q.MoveMovieFile(ctx, database.MoveMovieFileParams{
			NewPath: move.To, FileName: filepath.Base(move.To), Title: titleYear.Title, Year: helpers.NullInt64(int64(titleYear.Year)),
			ID: move.From.ID, FilePath: move.From.Path,
		})
		if err != nil || rows == 0 {
			return false, err
		}
		if searchAgain {
			err = q.ClearMovieTmdbRetry(ctx, move.From.ID)
		}
		return true, err
	}, func() {
		s.invalidateCommittedMovie(move.From.ID)
	})
	contextErr := ctx.Err()
	if contextErr != nil {
		return false
	}
	if err != nil {
		s.logger.Warn("movie move failed; the file is imported as new", "from", move.From.Path, "to", move.To, "error", err)
		return false
	}
	if !moved {
		return false
	}
	scan.deleteEntry(old)
	entry.FilePath = move.To
	if searchAgain {
		entry.PendingRetry, entry.RetryAttempts, entry.LastAttemptAt = false, 0, sql.NullInt64{}
	}
	scan.setEntry(move.To, entry)
	s.logger.Debug("movie file moved", "from", move.From.Path, "to", move.To)
	return true
}
