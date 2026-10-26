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
// what its old and new paths say of it.
type showMove struct {
	scanner.Move
	before, local localEpisodeFile
}

// applyMoves re-keys each episode file the walk found under a path the
// catalog does not hold, when it is a cataloged file renamed or moved within
// its season, so it keeps its row and episode links. A renamed show folder
// keeps its show too, and with it its seasons, episodes and their watch
// progress, once the old folder is gone and the new one has no show of its
// own; every file that left a show folder must have gone to the same new one.
// A file moved to another season, and a show folder split in two, are
// imported anew and their old rows cleaned up, as before. Processing then
// checks each moved file like any other; one renamed to other episode numbers
// is processed again regardless, which links it to them.
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
		groups[before.showPath] = append(groups[before.showPath], showMove{Move: move, before: before, local: after})
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
			followed := s.moveShow(ctx, folder, moves[0].before, target)
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
// since a rename is how a wrong or missing match is fixed. So is a matched
// show whose folder now names another title or year: shows have no Identify,
// so renaming the folder is how a wrong match is fixed, and its seasons and
// episodes forget their matches with it, which a new match would contradict.
func (s *Scanner) moveShow(ctx context.Context, folder string, before, target localEpisodeFile) bool {
	_, err := os.Stat(folder)
	gone := errors.Is(err, fs.ErrNotExist)
	if !gone {
		return false
	}
	renamed := before.title != target.title || before.year != target.year
	err = s.tx.Run(ctx, func(q *database.Queries) error {
		show, err := q.MoveShowDirectory(ctx, database.MoveShowDirectoryParams{NewPath: target.showPath, LocalName: target.title, PremiereYear: helpers.NullInt64(int64(target.year)), DirectoryPath: folder})
		if err != nil {
			return err
		}
		rematch := show.TmdbID.Valid && renamed
		if rematch {
			err = forgetShowMatch(ctx, q, show.ID)
			if err != nil {
				return err
			}
		}
		if !show.TmdbID.Valid || rematch {
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

// forgetShowMatch clears a show's TMDB identity, and its seasons' and
// episodes', and queues all of them for enrichment.
func forgetShowMatch(ctx context.Context, q *database.Queries, showID int64) error {
	for _, forget := range []func(context.Context, int64) error{
		q.ForgetShowMatch, q.ForgetShowSeasonMatches, q.ForgetShowEpisodeMatches, q.MarkShowSeasonsRetry, q.MarkShowEpisodesRetry,
	} {
		err := forget(ctx, showID)
		if err != nil {
			return err
		}
	}
	return nil
}

// moveFile re-keys one episode file.
func (s *Scanner) moveFile(ctx context.Context, scan *showScanContext, report *scanReport, reconciliation *scanner.Reconciliation, move showMove) bool {
	old := filepath.Clean(move.From.Path)
	entry := scan.index[old]
	moved, err := reconciliation.MoveConfirmed(ctx, s.tx, move.Move, &report.Report, func(q *database.Queries) (bool, error) {
		rows, err := q.MoveShowFile(ctx, database.MoveShowFileParams{NewPath: move.To, FileName: filepath.Base(move.To), ID: move.From.ID, FilePath: move.From.Path})
		return rows > 0, err
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
	// Episode links follow the file name, and only processing writes them.
	relink := !slices.Equal(move.before.episodes, move.local.episodes)
	if relink {
		entry.FileFingerprint = scanner.FileFingerprint{}
	}
	scan.index[move.To] = entry
	s.Logger.Debug("episode file moved", "from", move.From.Path, "to", move.To)
	return true
}
