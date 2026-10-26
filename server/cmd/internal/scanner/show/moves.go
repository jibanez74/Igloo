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
	scanner.Move
	local localEpisodeFile
}

// applyMoves re-keys each episode file the walk found under a path the
// catalog does not hold, when it is a cataloged file renamed or moved within
// its season, so it keeps its row and episode links. A renamed show folder
// keeps its show too, and with it its seasons, episodes and their watch
// progress, once the old folder is gone and the new one has no show of its
// own; every file that left a show folder must have gone to the same new one.
// A file moved to another season, a show folder split in two, and a matched
// show whose folder now names another title or year are imported anew and
// their old rows cleaned up, as before. A move records the file's fingerprint
// at its new path, so processing finds it unchanged.
func (s *Scanner) applyMoves(ctx context.Context, scan *showScanContext, report *scanReport, reconciliation *scanner.Reconciliation, root string, files []scanner.ScanFile) {
	groups := make(map[string][]showMove)
	var folders []string
	for _, file := range files {
		_, cataloged := scan.index[file.Path]
		if cataloged {
			continue
		}
		move, found := reconciliation.MovedFrom(file.Path)
		if !found {
			continue
		}
		before, err := parseFile(root, filepath.Clean(move.From.Path))
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
		groups[before.showPath] = append(groups[before.showPath], showMove{Move: move, local: after})
	}
	moved := 0
	for _, folder := range folders {
		contextErr := ctx.Err()
		if contextErr != nil {
			return
		}
		moves := groups[folder]
		target := moves[0].local
		split := slices.ContainsFunc(moves, func(move showMove) bool { return move.local.showPath != target.showPath })
		if split {
			continue
		}
		renamed := target.showPath != folder
		if renamed {
			followed := s.moveShow(ctx, folder, target)
			if !followed {
				continue
			}
		}
		for _, move := range moves {
			committed := s.moveFile(ctx, scan, report, reconciliation, move)
			if committed {
				moved++
			}
		}
	}
	if moved > 0 {
		s.Logger.Info("episode files moved", "files", moved)
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
	// No show had the old folder, a matched one is named anew, or the new
	// folder already has a show.
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
func (s *Scanner) moveFile(ctx context.Context, scan *showScanContext, report *scanReport, reconciliation *scanner.Reconciliation, move showMove) bool {
	old := filepath.Clean(move.From.Path)
	entry := scan.index[old]
	f := move.Fingerprint
	moved, err := reconciliation.MoveConfirmed(ctx, s.tx, move.Move, &report.Report, func(q *database.Queries) (bool, error) {
		rows, err := q.MoveShowFile(ctx, database.MoveShowFileParams{NewPath: move.To, FileName: filepath.Base(move.To), ID: move.From.ID, FilePath: move.From.Path})
		if err != nil || rows == 0 {
			return false, err
		}
		err = q.UpsertShowFingerprint(ctx, database.UpsertShowFingerprintParams{FileID: move.From.ID, MtimeNs: f.MtimeNS, CtimeNs: f.CtimeNS, Device: f.Device, Inode: f.Inode})
		return err == nil, err
	}, func() {
		s.InvalidateCommittedShowFile(entry.ID, entry.Episodes)
	})
	contextErr := ctx.Err()
	if contextErr != nil {
		return false
	}
	if err != nil {
		s.Logger.Warn("episode move failed; the file is imported as new", "from", move.From.Path, "to", move.To, "error", err)
		return false
	}
	if !moved {
		return false
	}
	delete(scan.index, old)
	entry.FilePath = move.To
	entry.FileFingerprint = f
	entry.HasFingerprint = true
	scan.index[move.To] = entry
	s.Logger.Debug("episode file moved", "from", move.From.Path, "to", move.To)
	return true
}
