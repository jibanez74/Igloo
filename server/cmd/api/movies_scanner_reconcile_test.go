package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

type stubMovieScannerFfprobe struct {
	result  *ffprobe.FfprobeResult
	results []*ffprobe.FfprobeResult
	err     error
	errs    []error
	calls   int
}

func (s *stubMovieScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	callIndex := s.calls
	s.calls++
	if callIndex < len(s.errs) && s.errs[callIndex] != nil {
		return nil, s.errs[callIndex]
	}
	if callIndex < len(s.results) && s.results[callIndex] != nil {
		return s.results[callIndex], nil
	}
	return s.result, s.err
}

func (s *stubMovieScannerFfprobe) GetAudioMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	return s.GetMetadata(filePath)
}

type stubMovieScannerTmdb struct {
	searchErr     error
	detailErr     error
	theatersErr   error
	searchResults []tmdb.TmdbMovie
	detailMovies  map[int]tmdb.TmdbMovie
	theaterMovies []*tmdb.TmdbMovie
	searchCalls   []stubMovieScannerTmdbSearchCall
	detailCalls   []int
	clearCalls    int
}

type stubMovieScannerTmdbSearchCall struct {
	title string
	year  []int
}

func (s *stubMovieScannerTmdb) GetTmdbMovieByID(_ context.Context, movie *tmdb.TmdbMovie) error {
	s.detailCalls = append(s.detailCalls, movie.TmdbID)
	if s.detailErr != nil {
		return s.detailErr
	}
	if s.detailMovies == nil {
		return errors.New("tmdb details unavailable")
	}
	details, ok := s.detailMovies[movie.TmdbID]
	if !ok {
		return errors.New("tmdb details unavailable")
	}
	*movie = details
	return nil
}

func (s *stubMovieScannerTmdb) SearchMoviesByTitleAndYear(_ context.Context, title string, year ...int) ([]tmdb.TmdbMovie, error) {
	yearCopy := append([]int(nil), year...)
	s.searchCalls = append(s.searchCalls, stubMovieScannerTmdbSearchCall{title: title, year: yearCopy})
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	results := make([]tmdb.TmdbMovie, len(s.searchResults))
	copy(results, s.searchResults)
	return results, nil
}

func (s *stubMovieScannerTmdb) GetMoviesInTheaters(_ context.Context) ([]*tmdb.TmdbMovie, error) {
	if s.theatersErr != nil {
		return nil, s.theatersErr
	}
	return s.theaterMovies, nil
}

func (s *stubMovieScannerTmdb) ClearCache() {
	s.clearCalls++
}

func testMovieMetadata() *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
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
	}
}

func countMovieScannerRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestProcessMoviesBatchWithContextSkipsUnchangedWithoutFfprobe(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Unchanged.Movie.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	ffprobeStub := &stubMovieScannerFfprobe{result: testMovieMetadata()}
	app.Ffprobe = ffprobeStub

	scan := newMovieScanContext(map[string]int64{path: 5})
	scanned, skipped, errCount := app.processMoviesBatchWithContext(context.Background(), scan, []movieFile{
		{path: path, ext: "mkv", size: 5},
	})

	if scanned != 0 || skipped != 1 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/1/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 0 {
		t.Fatalf("expected unchanged movie to skip ffprobe, got %d calls", ffprobeStub.calls)
	}
}

func TestProcessMoviesBatchRollsBackInvalidMovieFile(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Audio.Only.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	app.Ffprobe = &stubMovieScannerFfprobe{
		result: &ffprobe.FfprobeResult{
			Format: ffprobe.Format{
				Duration:   "120",
				Size:       "5",
				FormatName: "matroska,webm",
			},
			Streams: []ffprobe.Stream{
				{
					Index:     0,
					CodecName: "aac",
					CodecType: "audio",
					Channels:  2,
				},
			},
		},
	}
	app.Tmdb = &stubMovieScannerTmdb{searchErr: errors.New("tmdb unavailable")}

	scanned, skipped, errCount := app.processMoviesBatchWithContext(context.Background(), newMovieScanContext(nil), []movieFile{
		{path: path, ext: "mkv", size: 5},
	})

	if scanned != 0 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/0/1", scanned, skipped, errCount)
	}
	if got := countMovieScannerRows(t, app.DB, "SELECT COUNT(*) FROM movies WHERE file_path = ?", path); got != 0 {
		t.Fatalf("expected invalid movie transaction to roll back, got %d movie rows", got)
	}
}

func TestRunMovieScanPreservesMissingMovieRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	missingPath := filepath.Join(moviesDir, "Missing.Movie.1999.mkv")
	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Missing Movie",
		FilePath:  missingPath,
		FileName:  filepath.Base(missingPath),
		Size:      7,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert missing movie: %v", err)
	}

	app.Settings = &database.Setting{MoviesDir: sql.NullString{String: moviesDir, Valid: true}}
	app.runMovieScan()

	_, err = app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("expected missing movie row to be preserved: %v", err)
	}
}

func TestProcessMoviesBatch_AcceptsConfiguredVideoExtensions(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Ffprobe = &stubMovieScannerFfprobe{result: testMovieMetadata()}
	app.Tmdb = &stubMovieScannerTmdb{searchErr: errors.New("tmdb unavailable")}

	moviesDir := t.TempDir()
	files := []movieFile{
		{path: filepath.Join(moviesDir, "Sample Movie (2020).mov"), ext: "mov", size: 5},
		{path: filepath.Join(moviesDir, "Sample Movie (2021).m4v"), ext: "m4v", size: 5},
		{path: filepath.Join(moviesDir, "Sample Movie (2022).webm"), ext: "webm", size: 5},
	}
	for _, file := range files {
		err := os.WriteFile(file.path, []byte("movie"), 0o644)
		if err != nil {
			t.Fatalf("write movie %s: %v", file.path, err)
		}
	}

	scanned, skipped, errCount, _ := app.processMoviesBatch(context.Background(), files)
	if scanned != len(files) || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want %d/0/0", scanned, skipped, errCount, len(files))
	}

	for _, file := range files {
		var container string
		err := app.DB.QueryRowContext(context.Background(), `
			SELECT container
			FROM movies
			WHERE file_path = ?
			LIMIT 1
		`, file.path).Scan(&container)
		if err != nil {
			t.Fatalf("get movie %s: %v", file.path, err)
		}
		if container != file.ext {
			t.Fatalf("movie container = %q, want %q", container, file.ext)
		}
	}
}

func TestReconcileMissingMoviesDeletesStaleRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	existingPath := filepath.Join(moviesDir, "Casino.Royale.2006.mkv")
	err := os.WriteFile(existingPath, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	existingMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Casino Royale",
		FilePath:  existingPath,
		FileName:  filepath.Base(existingPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert existing movie: %v", err)
	}

	deletedPath := filepath.Join(moviesDir, "Deleted.Movie.1999.mkv")
	deletedMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Deleted Movie",
		FilePath:  deletedPath,
		FileName:  filepath.Base(deletedPath),
		Size:      7,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert deleted movie: %v", err)
	}

	deletedCount, renamedCount, err := app.reconcileMissingMovies(ctx, moviesDir, map[string]bool{
		filepath.Clean(existingPath): true,
	}, map[string]bool{})
	if err != nil {
		t.Fatalf("reconcileMissingMovies: %v", err)
	}
	if deletedCount != 1 {
		t.Fatalf("expected 1 deleted movie, got %d", deletedCount)
	}
	if renamedCount != 0 {
		t.Fatalf("expected 0 renamed movies, got %d", renamedCount)
	}

	_, err = app.Queries.GetMovieByID(ctx, existingMovie.ID)
	if err != nil {
		t.Fatalf("expected existing movie to remain: %v", err)
	}

	_, err = app.Queries.GetMovieByID(ctx, deletedMovie.ID)
	if err == nil {
		t.Fatal("expected deleted movie row to be removed")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestReconcileMissingMoviesPreservesRenamedMovieID(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &stubMovieScannerFfprobe{
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

	ctx := context.Background()
	moviesDir := t.TempDir()

	oldPath := filepath.Join(moviesDir, "Casino.Royale.2006.mkv")
	newPath := filepath.Join(moviesDir, "Casino Royale (2006).mkv")

	err := os.WriteFile(newPath, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write renamed file: %v", err)
	}

	originalMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Casino Royale",
		FilePath:  oldPath,
		FileName:  filepath.Base(oldPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		TmdbID:    helpers.NullInt64(36557),
		Year:      helpers.NullInt64(2006),
		Duration:  helpers.NullFloat64(120),
	})
	if err != nil {
		t.Fatalf("insert original movie: %v", err)
	}

	renamedMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Casino Royale",
		FilePath:  newPath,
		FileName:  filepath.Base(newPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		TmdbID:    helpers.NullInt64(36557),
		Year:      helpers.NullInt64(2006),
		Duration:  helpers.NullFloat64(120),
	})
	if err != nil {
		t.Fatalf("insert renamed movie: %v", err)
	}

	deletedCount, renamedCount, err := app.reconcileMissingMovies(ctx, moviesDir, map[string]bool{
		filepath.Clean(newPath): true,
	}, map[string]bool{
		filepath.Clean(newPath): true,
	})
	if err != nil {
		t.Fatalf("reconcileMissingMovies: %v", err)
	}
	if deletedCount != 0 {
		t.Fatalf("expected 0 deleted movies, got %d", deletedCount)
	}
	if renamedCount != 1 {
		t.Fatalf("expected 1 renamed movie, got %d", renamedCount)
	}

	preservedMovie, err := app.Queries.GetMovieByID(ctx, originalMovie.ID)
	if err != nil {
		t.Fatalf("expected original movie ID to remain: %v", err)
	}
	if preservedMovie.FilePath != newPath {
		t.Fatalf("expected original movie path to update to %q, got %q", newPath, preservedMovie.FilePath)
	}

	_, err = app.Queries.GetMovieByID(ctx, renamedMovie.ID)
	if err == nil {
		t.Fatal("expected temporary renamed movie row to be removed")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows for temporary row, got %v", err)
	}
}

func TestReconcileMissingMoviesPreservesRenamedMovieIDWithoutTmdb(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &stubMovieScannerFfprobe{
		result: &ffprobe.FfprobeResult{
			Format: ffprobe.Format{
				Duration:   "121",
				Size:       "5",
				FormatName: "matroska,webm",
			},
			Streams: []ffprobe.Stream{
				{
					Index:     0,
					CodecName: "h264",
					CodecType: "video",
					Width:     1280,
					Height:    720,
				},
			},
		},
	}

	ctx := context.Background()
	moviesDir := t.TempDir()

	oldPath := filepath.Join(moviesDir, "Moneyball.2011.mkv")
	newPath := filepath.Join(moviesDir, "Moneyball (2011) [Remastered].mkv")

	err := os.WriteFile(newPath, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write renamed file: %v", err)
	}

	originalMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball",
		FilePath:  oldPath,
		FileName:  filepath.Base(oldPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Year:      helpers.NullInt64(2011),
		Duration:  helpers.NullFloat64(121),
	})
	if err != nil {
		t.Fatalf("insert original movie: %v", err)
	}

	_, err = app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball",
		FilePath:  newPath,
		FileName:  filepath.Base(newPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Year:      helpers.NullInt64(2011),
		Duration:  helpers.NullFloat64(121),
	})
	if err != nil {
		t.Fatalf("insert renamed movie: %v", err)
	}

	deletedCount, renamedCount, err := app.reconcileMissingMovies(ctx, moviesDir, map[string]bool{
		filepath.Clean(newPath): true,
	}, map[string]bool{
		filepath.Clean(newPath): true,
	})
	if err != nil {
		t.Fatalf("reconcileMissingMovies: %v", err)
	}
	if deletedCount != 0 {
		t.Fatalf("expected 0 deleted movies, got %d", deletedCount)
	}
	if renamedCount != 1 {
		t.Fatalf("expected 1 renamed movie, got %d", renamedCount)
	}

	preservedMovie, err := app.Queries.GetMovieByID(ctx, originalMovie.ID)
	if err != nil {
		t.Fatalf("expected original movie ID to remain: %v", err)
	}
	if preservedMovie.FilePath != newPath {
		t.Fatalf("expected original movie path to update to %q, got %q", newPath, preservedMovie.FilePath)
	}
}

func TestMovieScannerUpsertOverwritesManualMetadata(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	path := "/movies/Moneyball.2011.mkv"

	_, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball",
		FilePath:  path,
		FileName:  "Moneyball.2011.mkv",
		Size:      100,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Overview:  helpers.NullString("Original overview"),
	})
	if err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	updated, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball Remastered",
		FilePath:  path,
		FileName:  "Moneyball.2011.mkv",
		Size:      200,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Overview:  helpers.NullString("Scanner overview"),
	})
	if err != nil {
		t.Fatalf("scanner upsert: %v", err)
	}

	if updated.Title != "Moneyball Remastered" {
		t.Fatalf("expected scanner title to overwrite manual title, got %q", updated.Title)
	}
	if !updated.Overview.Valid || updated.Overview.String != "Scanner overview" {
		t.Fatalf("expected scanner overview to overwrite manual overview, got %+v", updated.Overview)
	}
	if updated.Size != 200 {
		t.Fatalf("expected scanner-owned size to update to 200, got %d", updated.Size)
	}
}

func TestFindMovieRenameCandidateRejectsMismatchedTmdbID(t *testing.T) {
	missing := database.GetMovieScanIndexRow{
		ID:       1,
		Title:    "Casino Royale",
		FilePath: "/movies/Casino.Royale.2006.mkv",
		FileName: "Casino.Royale.2006.mkv",
		Size:     100,
		TmdbID:   helpers.NullInt64(36557),
		Year:     helpers.NullInt64(2006),
		Duration: helpers.NullFloat64(8640),
	}
	candidate := database.GetMovieScanIndexRow{
		ID:       2,
		Title:    "Casino Royale",
		FilePath: "/movies/Casino Royale (2006).mkv",
		FileName: "Casino Royale (2006).mkv",
		Size:     100,
		TmdbID:   helpers.NullInt64(999),
		Year:     helpers.NullInt64(2006),
		Duration: helpers.NullFloat64(8640),
	}

	processed := map[string]database.GetMovieScanIndexRow{
		filepath.Clean(candidate.FilePath): candidate,
	}
	got := findMovieRenameCandidate(missing, processed, buildMovieRenameIndex(processed))
	if got != nil {
		t.Fatalf("expected mismatched TMDB IDs to reject rename, got %+v", got)
	}
}

func TestFindMovieRenameCandidateTieBreaksByPath(t *testing.T) {
	missing := database.GetMovieScanIndexRow{
		ID:       1,
		Title:    "Moneyball",
		FilePath: "/movies/Moneyball.2011.mkv",
		FileName: "Moneyball.2011.mkv",
		Size:     100,
		Year:     helpers.NullInt64(2011),
		Duration: helpers.NullFloat64(7980),
	}
	first := database.GetMovieScanIndexRow{
		ID:       2,
		Title:    "Moneyball",
		FilePath: "/movies/b/Moneyball (2011).mkv",
		FileName: "Moneyball (2011).mkv",
		Size:     100,
		Year:     helpers.NullInt64(2011),
		Duration: helpers.NullFloat64(7980),
	}
	second := first
	second.ID = 3
	second.FilePath = "/movies/a/Moneyball (2011).mkv"

	processed := map[string]database.GetMovieScanIndexRow{
		filepath.Clean(first.FilePath):  first,
		filepath.Clean(second.FilePath): second,
	}
	got := findMovieRenameCandidate(missing, processed, buildMovieRenameIndex(processed))
	if got == nil {
		t.Fatal("expected a rename candidate")
	}
	if got.movie.ID != second.ID {
		t.Fatalf("candidate ID = %d, want lexicographically first path ID %d", got.movie.ID, second.ID)
	}
}

func TestReconcileMissingMoviesDeletesBelowThresholdRenameCandidate(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	oldPath := filepath.Join(moviesDir, "Old.Movie.2001.mkv")
	newPath := filepath.Join(moviesDir, "Different.Movie.2001.mkv")
	if err := os.WriteFile(newPath, []byte("movie"), 0o644); err != nil {
		t.Fatalf("write candidate file: %v", err)
	}

	staleMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Old Movie",
		FilePath:  oldPath,
		FileName:  filepath.Base(oldPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Year:      helpers.NullInt64(2001),
		Duration:  helpers.NullFloat64(100),
	})
	if err != nil {
		t.Fatalf("insert stale movie: %v", err)
	}
	candidateMovie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Different Movie",
		FilePath:  newPath,
		FileName:  filepath.Base(newPath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
		Year:      helpers.NullInt64(2001),
		Duration:  helpers.NullFloat64(999),
	})
	if err != nil {
		t.Fatalf("insert candidate movie: %v", err)
	}

	deletedCount, renamedCount, err := app.reconcileMissingMovies(ctx, moviesDir, map[string]bool{
		filepath.Clean(newPath): true,
	}, map[string]bool{
		filepath.Clean(newPath): true,
	})
	if err != nil {
		t.Fatalf("reconcileMissingMovies: %v", err)
	}
	if renamedCount != 0 {
		t.Fatalf("renamed count = %d, want 0", renamedCount)
	}
	if deletedCount != 1 {
		t.Fatalf("deleted count = %d, want 1", deletedCount)
	}

	_, err = app.Queries.GetMovieByID(ctx, staleMovie.ID)
	if err == nil {
		t.Fatal("expected below-threshold stale movie to be deleted")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows for deleted movie, got %v", err)
	}
	if _, err = app.Queries.GetMovieByID(ctx, candidateMovie.ID); err != nil {
		t.Fatalf("expected below-threshold candidate row to remain: %v", err)
	}
}

func TestReconcileMissingMoviesIgnoresRowsOutsideMoviesRoot(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "Outside.Movie.2002.mkv")

	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Outside Movie",
		FilePath:  outsidePath,
		FileName:  filepath.Base(outsidePath),
		Size:      5,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert outside movie: %v", err)
	}

	deletedCount, renamedCount, err := app.reconcileMissingMovies(ctx, moviesDir, map[string]bool{}, map[string]bool{})
	if err != nil {
		t.Fatalf("reconcileMissingMovies: %v", err)
	}
	if deletedCount != 0 || renamedCount != 0 {
		t.Fatalf("reconcile counts deleted=%d renamed=%d, want 0/0", deletedCount, renamedCount)
	}
	if _, err = app.Queries.GetMovieByID(ctx, movie.ID); err != nil {
		t.Fatalf("expected outside-root movie row to remain: %v", err)
	}
}
