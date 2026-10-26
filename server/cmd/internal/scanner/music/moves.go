package music

import (
	"context"
	"path/filepath"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/scanner"
)

// applyMoves re-keys each track the walk found under a path the catalog does
// not hold, when it is a cataloged file renamed or moved within the library,
// so it keeps its row: likes, playlist entries and its provider pairing. It
// runs on each batch before it is processed, so the rename is not imported
// as a new track. Processing then checks the file at its new path like any
// other: a renamed file's new ctime hashes it again, which records only the
// new fingerprint when its bytes are the same and reprocesses it when a
// tagger rewrote it, even with its modification time kept.
func (s *Scanner) applyMoves(ctx context.Context, scan *musicScanContext, report *scanReport, reconciliation *scanner.Reconciliation, files []scanner.ScanFile) {
	moved := 0
	for _, file := range files {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		path := filepath.Clean(file.Path)
		_, cataloged := scan.trackIndex[path]
		if cataloged {
			continue
		}
		move, found := reconciliation.MovedFrom(path)
		if !found {
			continue
		}
		committed, err := reconciliation.MoveConfirmed(ctx, s.tx, move, &report.Report, func(q *database.Queries) (bool, error) {
			rows, err := q.MoveTrackFile(ctx, database.MoveTrackFileParams{NewPath: path, FileName: filepath.Base(path), ID: move.From.ID, FilePath: move.From.Path})
			return rows > 0, err
		}, func() {
			s.invalidateCommittedTrack(move.From.ID)
		})
		contextErr = ctx.Err()
		if contextErr != nil {
			return
		}
		if err != nil {
			s.logger.Warn("track move failed; the file is imported as new", "from", move.From.Path, "to", path, "error", err)
			continue
		}
		if !committed {
			continue
		}
		old := filepath.Clean(move.From.Path)
		scan.trackIndex[path] = scan.trackIndex[old]
		delete(scan.trackIndex, old)
		moved++
		s.logger.Debug("track file moved", "from", move.From.Path, "to", path)
	}
	if moved > 0 {
		s.logger.Info("track files moved", "files", moved)
	}
}
