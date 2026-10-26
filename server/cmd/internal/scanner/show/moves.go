package show

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
)

// showMove is a cataloged episode file found again under a new path, with
// what that path says of it.
type showMove struct {
	from  scanner.CatalogFile
	to    string
	local localEpisodeFile
}

// applyMoves re-keys each episode file the walk found under a path the
// catalog does not hold, when it is a cataloged file renamed or moved within
// its season, so it keeps its row and episode links. A renamed show folder
// keeps its show too, and with it its seasons, episodes and their watch
// progress, once the old folder is gone and the new one has no show of its
// own; every file that left a show folder must have gone to the same new one.
// A file moved to another season, or a show folder split in two, is imported
// anew and its old row cleaned up, as before.
func (s *Scanner) applyMoves(ctx context.Context, scan *showScanContext, reconciliation *scanner.Reconciliation, root string, files []scanner.ScanFile) {
	groups := make(map[string][]showMove)
	var folders []string
	for _, file := range files {
		_, cataloged := scan.index[file.Path]
		if cataloged {
			continue
		}
		from, found := reconciliation.MovedFrom(file.Path)
		if !found {
			continue
		}
		before, err := parseFile(root, filepath.Clean(from.Path))
		if err != nil {
			continue
		}
		after, err := parseFile(root, file.Path)
		if err != nil || after.season != before.season {
			continue
		}
		_, grouped := groups[before.showPath]
		if !grouped {
			folders = append(folders, before.showPath)
		}
		groups[before.showPath] = append(groups[before.showPath], showMove{from: from, to: file.Path, local: after})
	}
	for _, folder := range folders {
		moves := groups[folder]
		target := moves[0].local
		split := slices.ContainsFunc(moves, func(move showMove) bool { return move.local.showPath != target.showPath })
		if split {
			continue
		}
		renamed := target.showPath != folder
		if renamed && !s.moveShow(ctx, folder, target) {
			continue
		}
		for _, move := range moves {
			s.moveFile(ctx, scan, reconciliation, move)
		}
	}
}

// moveShow re-keys the show of a renamed folder to its new folder. Its local
// name follows the folder, and an unmatched show is searched again at once,
// since a rename is how a wrong or missing match is fixed.
func (s *Scanner) moveShow(ctx context.Context, folder string, target localEpisodeFile) bool {
	_, err := os.Stat(folder)
	gone := errors.Is(err, fs.ErrNotExist)
	if !gone {
		return false
	}
	err = s.tx.Run(ctx, func(q *database.Queries) error {
		show, err := q.MoveShowDirectory(ctx, database.MoveShowDirectoryParams{NewPath: target.showPath, LocalName: target.title, PremiereYear: helpers.NullInt64(int64(target.year)), DirectoryPath: folder})
		if err != nil {
			return err
		}
		if !show.TmdbID.Valid {
			return q.MarkShowRetry(ctx, show.ID)
		}
		return nil
	}, nil)
	// No show had the old folder, or the new folder already has one.
	unmoved := errors.Is(err, sql.ErrNoRows)
	if unmoved {
		return false
	}
	if err != nil {
		s.Logger.Warn("show folder move failed; its files are imported as new", "from", folder, "to", target.showPath, "error", err)
		return false
	}
	s.Logger.Info("show folder moved", "from", folder, "to", target.showPath)
	return true
}

// moveFile re-keys one episode file.
func (s *Scanner) moveFile(ctx context.Context, scan *showScanContext, reconciliation *scanner.Reconciliation, move showMove) {
	old := filepath.Clean(move.from.Path)
	entry := scan.index[old]
	moved, err := reconciliation.MoveConfirmed(ctx, s.tx, move.from, func(q *database.Queries) (bool, error) {
		rows, err := q.MoveShowFile(ctx, database.MoveShowFileParams{NewPath: move.to, FileName: filepath.Base(move.to), ID: move.from.ID, FilePath: move.from.Path})
		return rows > 0, err
	}, func() {
		s.InvalidateCommittedShowFile(entry.ID, entry.Episodes)
	})
	if err != nil {
		s.Logger.Warn("episode move failed; the file is imported as new", "from", move.from.Path, "to", move.to, "error", err)
		return
	}
	if !moved {
		return
	}
	delete(scan.index, old)
	entry.FilePath = move.to
	scan.index[move.to] = entry
	s.Logger.Info("episode file moved", "from", move.from.Path, "to", move.to)
}
