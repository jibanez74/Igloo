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
// as a new track; the bytes are unchanged, so processing only records the new
// fingerprint.
func (s *Scanner) applyMoves(ctx context.Context, scan *musicScanContext, reconciliation *scanner.Reconciliation, files []scanner.ScanFile) {
	for _, file := range files {
		path := filepath.Clean(file.Path)
		_, cataloged := scan.trackIndex[path]
		if cataloged {
			continue
		}
		from, found := reconciliation.MovedFrom(path)
		if !found {
			continue
		}
		moved, err := reconciliation.MoveConfirmed(ctx, s.tx, from, func(q *database.Queries) (bool, error) {
			rows, err := q.MoveTrackFile(ctx, database.MoveTrackFileParams{NewPath: path, FileName: filepath.Base(path), ID: from.ID, FilePath: from.Path})
			return rows > 0, err
		}, func() {
			s.invalidateCommittedTrack(from.ID)
		})
		if err != nil {
			s.logger.Warn("track move failed; the file is imported as new", "from", from.Path, "to", path, "error", err)
			continue
		}
		if !moved {
			continue
		}
		old := filepath.Clean(from.Path)
		scan.trackIndex[path] = scan.trackIndex[old]
		delete(scan.trackIndex, old)
		s.logger.Info("track file moved", "from", from.Path, "to", path)
	}
}
