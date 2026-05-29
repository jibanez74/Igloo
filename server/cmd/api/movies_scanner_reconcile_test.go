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
	result *ffprobe.FfprobeResult
	err    error
}

func (s *stubMovieScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	return s.result, s.err
}

func (s *stubMovieScannerFfprobe) GetAudioMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	return s.result, s.err
}

type stubMovieScannerTmdb struct {
	searchErr error
}

func (s *stubMovieScannerTmdb) GetTmdbMovieByID(_ context.Context, _ *tmdb.TmdbMovie) error {
	return errors.New("tmdb details unavailable")
}

func (s *stubMovieScannerTmdb) SearchMoviesByTitleAndYear(_ context.Context, _ string, _ ...int) ([]tmdb.TmdbMovie, error) {
	return nil, s.searchErr
}

func (s *stubMovieScannerTmdb) GetMoviesInTheaters(_ context.Context) ([]*tmdb.TmdbMovie, error) {
	return nil, errors.New("tmdb theaters unavailable")
}

func (s *stubMovieScannerTmdb) ClearCache() {}

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
		movie, err := app.Queries.GetMovieByPath(context.Background(), file.path)
		if err != nil {
			t.Fatalf("get movie %s: %v", file.path, err)
		}
		if movie.Container != file.ext {
			t.Fatalf("movie container = %q, want %q", movie.Container, file.ext)
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

func TestMovieMetadataLocksPreventScannerOverwrite(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	path := "/movies/Moneyball.2011.mkv"

	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
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

	err = app.Queries.LockMovieMetadataFields(ctx, movieMetadataLocks{
		Title:    true,
		Overview: true,
	}.toParams(movie.ID))
	if err != nil {
		t.Fatalf("lock metadata: %v", err)
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

	if updated.Title != "Moneyball" {
		t.Fatalf("expected locked title to stay Moneyball, got %q", updated.Title)
	}
	if !updated.Overview.Valid || updated.Overview.String != "Original overview" {
		t.Fatalf("expected locked overview to stay original, got %+v", updated.Overview)
	}
	if updated.Size != 200 {
		t.Fatalf("expected scanner-owned size to update to 200, got %d", updated.Size)
	}
}
