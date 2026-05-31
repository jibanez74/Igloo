package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type movieFile struct {
	path string
	ext  string
	size int64
}

type movieRenameCandidate struct {
	movie database.GetMovieScanIndexRow
	score float64
}

type movieRenameIndex struct {
	byTmdbID         map[int64][]database.GetMovieScanIndexRow
	byTitleYear      map[string][]database.GetMovieScanIndexRow
	bySize           map[int64][]database.GetMovieScanIndexRow
	byDurationBucket map[int64][]database.GetMovieScanIndexRow
}

func (app *Application) ScanMoviesLibrary() {
	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		app.Logger.Error("movies directory not configured")
		return
	}

	if !tryBeginMovieScan() {
		app.Logger.Warn("movie library scan is already in progress")
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}
	go app.runMovieScan()
}

func (app *Application) runMovieScan() {
	if app.Wait != nil {
		defer app.Wait.Done()
	}
	defer finishMovieScan()

	if !app.Settings.MoviesDir.Valid || app.Settings.MoviesDir.String == "" {
		app.Logger.Error("movies directory not configured")
		return
	}

	app.Logger.Info(fmt.Sprintf("scanning movies directory: %s", app.Settings.MoviesDir.String))

	ctx := context.Background()
	errorCount := 0
	moviesScanned := 0
	moviesSkipped := 0
	startTime := time.Now()

	scanIndex, err := app.loadMovieScanIndex(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to load movie scan index: %s", err.Error()))
		return
	}
	scan := newMovieScanContext(scanIndex)

	batch := make([]movieFile, 0, helpers.SCANNER_BATCH_SIZE)

	err = filepath.WalkDir(app.Settings.MoviesDir.String, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == app.Settings.MoviesDir.String {
				return err
			}
			app.Logger.Error(fmt.Sprintf("error walking directory: %s", err.Error()))
			errorCount++
			return nil
		}

		if entry.IsDir() {
			return nil
		}

		ext := helpers.GetFileExtension(path)
		if !helpers.ValidVideoExtensions[ext] {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to get file info for %s: %s", path, err.Error()))
			errorCount++
			return nil
		}

		if scan.movieUnchanged(path, info.Size()) {
			moviesSkipped++
			return nil
		}

		batch = append(batch, movieFile{path: path, ext: ext, size: info.Size()})

		if len(batch) >= helpers.SCANNER_BATCH_SIZE {
			scanned, skipped, errors := app.processMoviesBatchWithContext(ctx, scan, batch)
			moviesScanned += scanned
			moviesSkipped += skipped
			errorCount += errors
			batch = batch[:0]
		}

		return nil
	})

	if err != nil {
		app.Logger.Error(fmt.Sprintf("unexpected error walking movies directory: %s", err.Error()))
		return
	}

	if len(batch) > 0 {
		scanned, skipped, errors := app.processMoviesBatchWithContext(ctx, scan, batch)
		moviesScanned += scanned
		moviesSkipped += skipped
		errorCount += errors
	}

	app.Logger.Info(fmt.Sprintf("movies scanner completed: %d scanned, %d skipped, %d errors in %s",
		moviesScanned, moviesSkipped, errorCount, helpers.FormatDuration(time.Since(startTime))))
}

func (app *Application) processMoviesBatch(ctx context.Context, files []movieFile) (scanned, skipped, errCount int, processed []string) {
	scanIndex, err := app.loadMovieScanIndex(ctx)
	if err != nil {
		app.Logger.Error(fmt.Sprintf("failed to load movie scan index: %s", err.Error()))
		return 0, 0, 1, nil
	}

	scan := newMovieScanContext(scanIndex)
	scanned, skipped, errCount = app.processMoviesBatchWithContext(ctx, scan, files)
	return scanned, skipped, errCount, nil
}

func (app *Application) processMoviesBatchWithContext(ctx context.Context, scan *movieScanContext, files []movieFile) (scanned, skipped, errCount int) {
	for _, file := range files {
		if scan.movieUnchanged(file.path, file.size) {
			skipped++
			continue
		}

		err := app.processMovie(ctx, scan, file)
		if err != nil {
			app.Logger.Error(fmt.Sprintf("failed to process %s: %s", file.path, err.Error()))
			errCount++
			continue
		}

		scanned++
	}

	return scanned, skipped, errCount
}

func (app *Application) processMovie(ctx context.Context, scan *movieScanContext, file movieFile) error {
	resolved, err := app.resolveMovieFile(ctx, file)
	if err != nil {
		return err
	}

	_, err = app.persistResolvedMovie(ctx, scan, resolved)
	return err
}

func (app *Application) persistResolvedMovie(ctx context.Context, scan *movieScanContext, resolved *resolvedMovie) (int64, error) {
	txScan := scan.clone()

	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)
	movieID, err := app.persistResolvedMovieTx(ctx, qtx, txScan, resolved)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("failed to commit movie: %w", err)
	}

	txScan.movieIndex[filepath.Clean(resolved.params.FilePath)] = resolved.fileSize
	scan.mergeFrom(txScan)

	return movieID, nil
}

func (app *Application) loadMovieScanIndex(ctx context.Context) (map[string]int64, error) {
	rows, err := app.Queries.GetMovieScanIndex(ctx)
	if err != nil {
		return nil, err
	}

	index := make(map[string]int64, len(rows))
	for _, row := range rows {
		index[filepath.Clean(row.FilePath)] = row.Size
	}

	return index, nil
}

func (app *Application) reconcileMissingMovies(
	ctx context.Context,
	moviesRoot string,
	scannedPaths map[string]bool,
	processedPaths map[string]bool,
) (deletedCount int, renamedCount int, err error) {
	app.ScannerDBMu.Lock()
	defer app.ScannerDBMu.Unlock()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	qtx := app.Queries.WithTx(tx)

	movies, err := qtx.GetMovieScanIndex(ctx)
	if err != nil {
		return 0, 0, err
	}

	missingMovies := make([]database.GetMovieScanIndexRow, 0)
	processedMoviesByPath := make(map[string]database.GetMovieScanIndexRow)
	for _, movie := range movies {
		cleanPath := filepath.Clean(movie.FilePath)
		if !processedPaths[cleanPath] {
			if isMovieUnderRoot(cleanPath, moviesRoot) && !scannedPaths[cleanPath] {
				missingMovies = append(missingMovies, movie)
			}
			continue
		}
		processedMoviesByPath[cleanPath] = movie
	}

	renameIndex := movieRenameIndex{}
	if len(missingMovies) > 0 && len(processedMoviesByPath) > 0 {
		renameIndex = buildMovieRenameIndex(processedMoviesByPath)
	}

	for _, movie := range missingMovies {
		cleanPath := filepath.Clean(movie.FilePath)
		_, statErr := os.Stat(cleanPath)
		if statErr == nil {
			continue
		}
		if !os.IsNotExist(statErr) {
			app.Logger.Warn("failed to stat movie during reconciliation", "path", cleanPath, "error", statErr)
			continue
		}

		savepointName := fmt.Sprintf("sp_reconcile_movie_%d", movie.ID)
		err = manageSavepoint(ctx, tx, savepointName, func() error {
			reconciled, reconcileErr := app.reconcileRenamedMovie(ctx, qtx, movie, processedMoviesByPath, renameIndex)
			if reconcileErr != nil {
				return reconcileErr
			}
			if reconciled {
				renamedCount++
				return nil
			}

			app.invalidateSubtitleVTTCache(movie.ID)

			deleteErr := qtx.DeleteMovie(ctx, movie.ID)
			if deleteErr != nil {
				return deleteErr
			}

			deletedCount++
			return nil
		})
		if err != nil {
			return deletedCount, renamedCount, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return deletedCount, renamedCount, err
	}

	return deletedCount, renamedCount, nil
}

func (app *Application) reconcileRenamedMovie(
	ctx context.Context,
	qtx *database.Queries,
	missingMovie database.GetMovieScanIndexRow,
	processedMoviesByPath map[string]database.GetMovieScanIndexRow,
	renameIndex movieRenameIndex,
) (bool, error) {
	candidate := findMovieRenameCandidate(missingMovie, processedMoviesByPath, renameIndex)
	if candidate == nil {
		return false, nil
	}

	candidatePath := filepath.Clean(candidate.movie.FilePath)
	info, err := os.Stat(candidatePath)
	if err != nil {
		if os.IsNotExist(err) {
			delete(processedMoviesByPath, candidatePath)
			return false, nil
		}
		return false, err
	}

	app.invalidateSubtitleVTTCache(candidate.movie.ID)
	err = qtx.DeleteMovie(ctx, candidate.movie.ID)
	if err != nil {
		return false, err
	}

	app.invalidateSubtitleVTTCache(missingMovie.ID)
	err = qtx.ReassignMoviePath(ctx, database.ReassignMoviePathParams{
		FilePath: candidatePath,
		FileName: filepath.Base(candidatePath),
		ID:       missingMovie.ID,
	})
	if err != nil {
		return false, err
	}

	ext := helpers.GetFileExtension(candidatePath)
	err = app.processMovieFile(ctx, qtx, candidatePath, ext, info.Size())
	if err != nil {
		return false, err
	}

	delete(processedMoviesByPath, candidatePath)
	removeMovieRenameCandidate(renameIndex, candidate.movie)
	return true, nil
}

func buildMovieRenameIndex(processedMoviesByPath map[string]database.GetMovieScanIndexRow) movieRenameIndex {
	index := movieRenameIndex{
		byTmdbID:         make(map[int64][]database.GetMovieScanIndexRow),
		byTitleYear:      make(map[string][]database.GetMovieScanIndexRow),
		bySize:           make(map[int64][]database.GetMovieScanIndexRow),
		byDurationBucket: make(map[int64][]database.GetMovieScanIndexRow),
	}

	for _, movie := range processedMoviesByPath {
		if movie.TmdbID.Valid {
			index.byTmdbID[movie.TmdbID.Int64] = append(index.byTmdbID[movie.TmdbID.Int64], movie)
		}

		titleYearKey := movieRenameTitleYearKey(movie)
		if titleYearKey != "" {
			index.byTitleYear[titleYearKey] = append(index.byTitleYear[titleYearKey], movie)
		}

		index.bySize[movie.Size] = append(index.bySize[movie.Size], movie)

		durationBucket, ok := movieRenameDurationBucket(movie)
		if ok {
			index.byDurationBucket[durationBucket] = append(index.byDurationBucket[durationBucket], movie)
		}
	}

	return index
}

func findMovieRenameCandidate(
	missingMovie database.GetMovieScanIndexRow,
	processedMoviesByPath map[string]database.GetMovieScanIndexRow,
	renameIndex movieRenameIndex,
) *movieRenameCandidate {
	candidates := renameIndex.lookupCandidates(missingMovie)
	if len(candidates) == 0 {
		return nil
	}

	var best *movieRenameCandidate
	for _, candidateMovie := range candidates {
		if candidateMovie.ID == missingMovie.ID {
			continue
		}
		if _, ok := processedMoviesByPath[filepath.Clean(candidateMovie.FilePath)]; !ok {
			continue
		}

		score := scoreMovieRenameCandidate(missingMovie, candidateMovie)
		if score < helpers.MOVIE_RENAME_MATCH_THRESHOLD {
			continue
		}

		if best == nil || score > best.score {
			best = &movieRenameCandidate{
				movie: candidateMovie,
				score: score,
			}
			continue
		}

		if score == best.score && candidateMovie.FilePath < best.movie.FilePath {
			best = &movieRenameCandidate{
				movie: candidateMovie,
				score: score,
			}
		}
	}

	return best
}

func scoreMovieRenameCandidate(missingMovie, candidateMovie database.GetMovieScanIndexRow) float64 {
	score := 0.0

	if missingMovie.TmdbID.Valid && candidateMovie.TmdbID.Valid {
		if missingMovie.TmdbID.Int64 != candidateMovie.TmdbID.Int64 {
			return 0
		}
		score += helpers.MOVIE_RENAME_TMDB_ID_SCORE
	}

	missingTitle := movieRenameTitle(missingMovie)
	candidateTitle := movieRenameTitle(candidateMovie)
	switch {
	case missingTitle != "" && missingTitle == candidateTitle:
		score += helpers.MOVIE_RENAME_TITLE_SCORE
	case missingTitle != "" && candidateTitle != "" && tokenOverlapScore(missingTitle, candidateTitle) >= 0.75:
		score += helpers.MOVIE_RENAME_TITLE_SCORE / 2
	}

	if missingMovie.Year.Valid && candidateMovie.Year.Valid && missingMovie.Year.Int64 == candidateMovie.Year.Int64 {
		score += helpers.MOVIE_RENAME_YEAR_SCORE
	}

	if missingMovie.Size == candidateMovie.Size {
		score += helpers.MOVIE_RENAME_SIZE_SCORE
	}

	if missingMovie.Duration.Valid && candidateMovie.Duration.Valid {
		durationDiff := missingMovie.Duration.Float64 - candidateMovie.Duration.Float64
		if durationDiff < 0 {
			durationDiff = -durationDiff
		}
		if durationDiff <= 5 {
			score += helpers.MOVIE_RENAME_DURATION_SCORE
		}
	}

	return score
}

func movieRenameTitle(movie database.GetMovieScanIndexRow) string {
	parsed, err := helpers.GetTitleAndYearFromFileName(movie.FileName)
	if err == nil && parsed.Title != "" {
		return normalizeComparableMovieTitle(parsed.Title)
	}

	return normalizeComparableMovieTitle(movie.Title)
}

func movieRenameTitleYearKey(movie database.GetMovieScanIndexRow) string {
	title := movieRenameTitle(movie)
	if title == "" {
		return ""
	}
	if movie.Year.Valid {
		return fmt.Sprintf("%s|%d", title, movie.Year.Int64)
	}
	return title
}

func movieRenameDurationBucket(movie database.GetMovieScanIndexRow) (int64, bool) {
	if !movie.Duration.Valid {
		return 0, false
	}
	return int64(movie.Duration.Float64 / 10), true
}

func removeMovieRenameCandidate(index movieRenameIndex, movie database.GetMovieScanIndexRow) {
	if movie.TmdbID.Valid {
		index.byTmdbID[movie.TmdbID.Int64] = removeMovieRenameRow(index.byTmdbID[movie.TmdbID.Int64], movie.ID)
	}

	titleYearKey := movieRenameTitleYearKey(movie)
	if titleYearKey != "" {
		index.byTitleYear[titleYearKey] = removeMovieRenameRow(index.byTitleYear[titleYearKey], movie.ID)
	}

	index.bySize[movie.Size] = removeMovieRenameRow(index.bySize[movie.Size], movie.ID)

	durationBucket, ok := movieRenameDurationBucket(movie)
	if ok {
		index.byDurationBucket[durationBucket] = removeMovieRenameRow(index.byDurationBucket[durationBucket], movie.ID)
	}
}

func removeMovieRenameRow(rows []database.GetMovieScanIndexRow, movieID int64) []database.GetMovieScanIndexRow {
	for i := range rows {
		if rows[i].ID != movieID {
			continue
		}
		return append(rows[:i], rows[i+1:]...)
	}
	return rows
}

func (index movieRenameIndex) lookupCandidates(missingMovie database.GetMovieScanIndexRow) []database.GetMovieScanIndexRow {
	candidateMap := make(map[int64]database.GetMovieScanIndexRow)
	appendCandidates := func(rows []database.GetMovieScanIndexRow) {
		for _, movie := range rows {
			candidateMap[movie.ID] = movie
		}
	}

	if missingMovie.TmdbID.Valid {
		appendCandidates(index.byTmdbID[missingMovie.TmdbID.Int64])
	}

	titleYearKey := movieRenameTitleYearKey(missingMovie)
	if titleYearKey != "" {
		appendCandidates(index.byTitleYear[titleYearKey])
	}

	appendCandidates(index.bySize[missingMovie.Size])

	durationBucket, ok := movieRenameDurationBucket(missingMovie)
	if ok {
		appendCandidates(index.byDurationBucket[durationBucket])
	}

	if len(candidateMap) == 0 {
		return nil
	}

	candidates := make([]database.GetMovieScanIndexRow, 0, len(candidateMap))
	for _, movie := range candidateMap {
		candidates = append(candidates, movie)
	}

	return candidates
}

func isMovieUnderRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == "" || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func tryBeginMovieScan() bool {
	movieScanMutex.Lock()
	defer movieScanMutex.Unlock()

	if isMovieScanning {
		return false
	}

	isMovieScanning = true
	return true
}

func finishMovieScan() {
	movieScanMutex.Lock()
	isMovieScanning = false
	movieScanMutex.Unlock()
}
