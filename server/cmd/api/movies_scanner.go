package main

import (
	"context"
	"fmt"
	"igloo/cmd/internal/helpers"
	"io/fs"
	"path/filepath"
	"time"
)

type movieFile struct {
	path string
	ext  string
	size int64
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

	// movieIndex is shared (never written inside the transaction) and is only
	// updated here, after a successful commit, so a movie whose transaction
	// failed is never recorded as scanned/unchanged.
	scan.movieIndex[filepath.Clean(resolved.params.FilePath)] = resolved.fileSize
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
