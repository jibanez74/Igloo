package show

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"

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

// errShowRenamedAway rolls back the move of a matched show whose new folder
// name contradicts its match.
var errShowRenamedAway = errors.New("matched show renamed to another title or year")

// moveShow re-keys the show of a renamed folder to its new folder. Its local
// name follows the folder, and an unmatched show is searched again at once,
// since a rename is how a wrong or missing match is fixed. A matched show
// moves while the new name agrees with its match: the same title, and the
// same year, no year, or a year added that its first air date bears out, as
// Sonarr's {Series TitleYear} folders add. Any other rename of a matched show
// is how a wrong match is fixed, since shows have no Identify, so its files
// are imported as a new show and searched again, as before.
func (s *Scanner) moveShow(ctx context.Context, folder string, before, target localEpisodeFile) bool {
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
		agrees := keepsMatch(before, target, show.FirstAirDate.String)
		if !agrees {
			return errShowRenamedAway
		}
		return nil
	}, nil)
	// No show had the old folder, the new folder already has one, or the new
	// name contradicts the show's match.
	unmoved := errors.Is(err, sql.ErrNoRows) || errors.Is(err, errShowRenamedAway)
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

// keepsMatch reports whether a matched show's renamed folder still names it:
// the same title, and the same year, no year, or a year added that its first
// air date (YYYY-MM-DD) bears out.
func keepsMatch(before, target localEpisodeFile, firstAirDate string) bool {
	if before.title != target.title {
		return false
	}
	airYear := ""
	if len(firstAirDate) >= 4 {
		airYear = firstAirDate[:4]
	}
	added := before.year == 0 && strconv.Itoa(target.year) == airYear
	return target.year == before.year || target.year == 0 || added
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
