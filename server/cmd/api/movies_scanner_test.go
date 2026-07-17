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
	return s.result, nil
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

func (*stubMovieScannerTmdb) ClearCache() {}

func testMovieMetadata() *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "120",
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
	scanned, skipped, errCount := app.processMoviesBatchWithContext(context.Background(), scan, []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
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
				Duration: "120",
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

	scanned, skipped, errCount := app.processMoviesBatchWithContext(context.Background(), newMovieScanContext(nil), []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
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

func TestRunMovieScan_AcceptsConfiguredVideoExtensions(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	files := []struct {
		path string
		ext  string
	}{
		{path: filepath.Join(moviesDir, "Sample Movie (2020).mov"), ext: "mov"},
		{path: filepath.Join(moviesDir, "Sample Movie (2021).m4v"), ext: "m4v"},
		{path: filepath.Join(moviesDir, "Sample Movie (2022).webm"), ext: "webm"},
	}
	for _, file := range files {
		err := os.WriteFile(file.path, []byte("movie"), 0o644)
		if err != nil {
			t.Fatalf("write movie %s: %v", file.path, err)
		}
	}

	ffprobeStub := &stubMovieScannerFfprobe{result: testMovieMetadata()}
	app.Ffprobe = ffprobeStub
	app.Settings = &database.Setting{MoviesDir: sql.NullString{String: moviesDir, Valid: true}}

	app.runMovieScan()

	if ffprobeStub.calls != len(files) {
		t.Fatalf("ffprobe calls = %d, want %d", ffprobeStub.calls, len(files))
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
