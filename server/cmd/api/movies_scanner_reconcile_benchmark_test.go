package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	applogger "igloo/cmd/internal/logger"

	_ "github.com/mattn/go-sqlite3"
	"github.com/patrickmn/go-cache"
)

func BenchmarkReconcileMissingMovies_NoMissingRows(b *testing.B) {
	for i := 0; i < b.N; i++ {
		app, moviesDir, scannedPaths, processedPaths := setupMovieReconcileBenchmarkState(b, 2000, 0, 0)

		b.StartTimer()
		_, _, err := app.reconcileMissingMovies(context.Background(), moviesDir, scannedPaths, processedPaths)
		b.StopTimer()

		if err != nil {
			b.Fatalf("reconcileMissingMovies: %v", err)
		}

		closeBenchmarkApp(b, app)
	}
}

func BenchmarkReconcileMissingMovies_ModerateRenameChurn(b *testing.B) {
	for i := 0; i < b.N; i++ {
		app, moviesDir, scannedPaths, processedPaths := setupMovieReconcileBenchmarkState(b, 1000, 48, 48)

		b.StartTimer()
		_, _, err := app.reconcileMissingMovies(context.Background(), moviesDir, scannedPaths, processedPaths)
		b.StopTimer()

		if err != nil {
			b.Fatalf("reconcileMissingMovies: %v", err)
		}

		closeBenchmarkApp(b, app)
	}
}

func BenchmarkReconcileMissingMovies_LargeLibrarySmallRenameSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		app, moviesDir, scannedPaths, processedPaths := setupMovieReconcileBenchmarkState(b, 5000, 24, 24)

		b.StartTimer()
		_, _, err := app.reconcileMissingMovies(context.Background(), moviesDir, scannedPaths, processedPaths)
		b.StopTimer()

		if err != nil {
			b.Fatalf("reconcileMissingMovies: %v", err)
		}

		closeBenchmarkApp(b, app)
	}
}

func BenchmarkReconcileMissingMovies_LargeLibraryLargeRenameSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		app, moviesDir, scannedPaths, processedPaths := setupMovieReconcileBenchmarkState(b, 5000, 500, 500)

		b.StartTimer()
		_, _, err := app.reconcileMissingMovies(context.Background(), moviesDir, scannedPaths, processedPaths)
		b.StopTimer()

		if err != nil {
			b.Fatalf("reconcileMissingMovies: %v", err)
		}

		closeBenchmarkApp(b, app)
	}
}

func BenchmarkFindMovieRenameCandidate(b *testing.B) {
	processedMoviesByPath, missingMovie := buildMovieRenameBenchmarkDataset(5000, 500)
	index := buildMovieRenameIndex(processedMoviesByPath)

	b.Run("indexed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			candidate := findMovieRenameCandidate(missingMovie, processedMoviesByPath, index)
			if candidate == nil {
				b.Fatal("expected rename candidate")
			}
		}
	})

	b.Run("legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			candidate := findMovieRenameCandidateLegacy(missingMovie, processedMoviesByPath)
			if candidate == nil {
				b.Fatal("expected rename candidate")
			}
		}
	})
}

func setupMovieReconcileBenchmarkState(
	b *testing.B,
	stableCount int,
	renameCount int,
	deletedCount int,
) (*Application, string, map[string]bool, map[string]bool) {
	b.Helper()

	app := setupBenchmarkApp(b)
	app.Ffprobe = benchmarkMovieScannerFfprobe()

	ctx := context.Background()
	moviesDir := b.TempDir()
	scannedPaths := make(map[string]bool, stableCount+renameCount)
	processedPaths := make(map[string]bool, renameCount)

	insertBenchmarkMovies(b, ctx, app, moviesDir, stableCount, renameCount, deletedCount, scannedPaths, processedPaths)

	return app, moviesDir, scannedPaths, processedPaths
}

func insertBenchmarkMovies(
	b *testing.B,
	ctx context.Context,
	app *Application,
	moviesDir string,
	stableCount int,
	renameCount int,
	deletedCount int,
	scannedPaths map[string]bool,
	processedPaths map[string]bool,
) {
	b.Helper()

	for i := 0; i < stableCount; i++ {
		fileName := fmt.Sprintf("Stable.Movie.%04d.2006.mkv", i)
		path := filepath.Join(moviesDir, fileName)
		err := os.WriteFile(path, []byte("movie"), 0o644)
		if err != nil {
			b.Fatalf("write stable file: %v", err)
		}

		insertBenchmarkMovieRow(b, ctx, app, path, fileName, int64(200000+i), 2006, int64(100+i), int64(7300+i))
		scannedPaths[filepath.Clean(path)] = true
	}

	for i := 0; i < renameCount; i++ {
		oldName := fmt.Sprintf("Renamed.Movie.%04d.2011.mkv", i)
		newName := fmt.Sprintf("Renamed Movie %04d (2011).mkv", i)
		oldPath := filepath.Join(moviesDir, oldName)
		newPath := filepath.Join(moviesDir, newName)

		err := os.WriteFile(newPath, []byte("movie"), 0o644)
		if err != nil {
			b.Fatalf("write renamed file: %v", err)
		}

		tmdbID := int64(900000 + i)
		insertBenchmarkMovieRow(b, ctx, app, oldPath, oldName, tmdbID, 2011, int64(200+i), int64(8100+i))
		insertBenchmarkMovieRow(b, ctx, app, newPath, newName, tmdbID, 2011, int64(200+i), int64(8100+i))

		cleanNewPath := filepath.Clean(newPath)
		scannedPaths[cleanNewPath] = true
		processedPaths[cleanNewPath] = true
	}

	for i := 0; i < deletedCount; i++ {
		fileName := fmt.Sprintf("Deleted.Movie.%04d.1999.mkv", i)
		path := filepath.Join(moviesDir, fileName)
		insertBenchmarkMovieRow(b, ctx, app, path, fileName, int64(600000+i), 1999, int64(50+i), int64(6900+i))
	}
}

func insertBenchmarkMovieRow(
	b *testing.B,
	ctx context.Context,
	app *Application,
	path string,
	fileName string,
	tmdbID int64,
	year int64,
	size int64,
	duration int64,
) {
	b.Helper()

	params := database.UpsertMovieParams{
		Title:     normalizeMovieTitleForSearch(fileName),
		FilePath:  path,
		FileName:  fileName,
		Size:      size,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		TmdbID:    helpers.NullInt64(tmdbID),
		Year:      helpers.NullInt64(year),
		Duration:  helpers.NullFloat64(float64(duration)),
	}

	_, err := app.Queries.UpsertMovie(ctx, params)
	if err != nil {
		b.Fatalf("insert benchmark movie %s: %v", fileName, err)
	}
}

func buildMovieRenameBenchmarkDataset(
	stableCount int,
	renameCount int,
) (map[string]database.GetMovieScanIndexRow, database.GetMovieScanIndexRow) {
	processed := make(map[string]database.GetMovieScanIndexRow, stableCount+renameCount)

	for i := 0; i < stableCount; i++ {
		path := filepath.Clean(fmt.Sprintf("/movies/Stable.Movie.%04d.2006.mkv", i))
		processed[path] = database.GetMovieScanIndexRow{
			ID:       int64(i + 1),
			Title:    "Stable Movie",
			FilePath: path,
			FileName: filepath.Base(path),
			Size:     int64(10000 + i),
			TmdbID:   helpers.NullInt64(int64(50000 + i)),
			Year:     helpers.NullInt64(2006),
			Duration: helpers.NullFloat64(7200 + float64(i)),
		}
	}

	targetID := int64(stableCount + 1000)
	targetPath := filepath.Clean("/movies/Casino Royale (2006).mkv")
	targetMovie := database.GetMovieScanIndexRow{
		ID:       targetID,
		Title:    "Casino Royale",
		FilePath: targetPath,
		FileName: filepath.Base(targetPath),
		Size:     123456,
		TmdbID:   helpers.NullInt64(36557),
		Year:     helpers.NullInt64(2006),
		Duration: helpers.NullFloat64(8640),
	}
	processed[targetPath] = targetMovie

	for i := 0; i < renameCount-1; i++ {
		path := filepath.Clean(fmt.Sprintf("/movies/Rename.Noise.%04d.2011.mkv", i))
		processed[path] = database.GetMovieScanIndexRow{
			ID:       int64(stableCount + i + 2),
			Title:    "Rename Noise",
			FilePath: path,
			FileName: filepath.Base(path),
			Size:     int64(123456 + i + 1),
			TmdbID:   helpers.NullInt64(int64(70000 + i)),
			Year:     helpers.NullInt64(2011),
			Duration: helpers.NullFloat64(8600 + float64(i)),
		}
	}

	missingMovie := database.GetMovieScanIndexRow{
		ID:       1_000_000,
		Title:    "Casino Royale",
		FilePath: "/movies/Casino.Royale.2006.mkv",
		FileName: "Casino.Royale.2006.mkv",
		Size:     123456,
		TmdbID:   helpers.NullInt64(36557),
		Year:     helpers.NullInt64(2006),
		Duration: helpers.NullFloat64(8640),
	}

	return processed, missingMovie
}

func findMovieRenameCandidateLegacy(
	missingMovie database.GetMovieScanIndexRow,
	processedMoviesByPath map[string]database.GetMovieScanIndexRow,
) *movieRenameCandidate {
	var best *movieRenameCandidate
	for _, candidateMovie := range processedMoviesByPath {
		if candidateMovie.ID == missingMovie.ID {
			continue
		}

		score := scoreMovieRenameCandidate(missingMovie, candidateMovie)
		if score < helpers.MOVIE_RENAME_MATCH_THRESHOLD {
			continue
		}

		if best == nil || score > best.score || (score == best.score && candidateMovie.FilePath < best.movie.FilePath) {
			best = &movieRenameCandidate{
				movie: candidateMovie,
				score: score,
			}
		}
	}

	return best
}

func benchmarkMovieScannerFfprobe() *stubMovieScannerFfprobe {
	return &stubMovieScannerFfprobe{
		result: &ffprobe.FfprobeResult{
			Format: ffprobe.Format{
				Duration:   "120",
				Size:       "5",
				FormatName: "matroska,webm",
			},
			Streams: []ffprobe.Stream{
				{
					Index:     0,
					CodecName: "h264",
					CodecType: "video",
					Width:     1920,
					Height:    1080,
				},
			},
		},
	}
}

func closeBenchmarkApp(b *testing.B, app *Application) {
	b.Helper()

	err := app.DB.Close()
	if err != nil {
		b.Fatalf("close benchmark db: %v", err)
	}
}

func setupBenchmarkApp(b *testing.B) *Application {
	b.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		b.Fatalf("open benchmark database: %v", err)
	}

	app := &Application{DB: db}
	logger, _, err := applogger.New(&applogger.LoggerConfig{
		Debug: true,
	})
	if err != nil {
		b.Fatalf("create benchmark logger: %v", err)
	}
	app.Logger = logger

	err = app.InitTables()
	if err != nil {
		b.Fatalf("InitTables failed: %v", err)
	}

	app.Queries, err = database.Prepare(context.Background(), db)
	if err != nil {
		b.Fatalf("prepare benchmark queries: %v", err)
	}

	app.HLSSessionCache = cache.New(helpers.HLS_SESSION_TTL, helpers.HLS_SESSION_CACHE_SWEEP)
	app.SubtitleVTTCache = cache.New(helpers.SUBTITLE_CACHE_TTL, helpers.SUBTITLE_CACHE_CLEANUP)

	return app
}
