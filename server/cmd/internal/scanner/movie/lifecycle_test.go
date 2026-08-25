package movie

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"

	_ "github.com/mattn/go-sqlite3"
)

func TestStartStatusesAndGuardRelease(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	notConfigured := testScanner.scanner.Start()
	if notConfigured.Status != StartNotConfigured {
		t.Fatalf("unconfigured Start status = %v, want %v", notConfigured.Status, StartNotConfigured)
	}

	testScanner.moviesDir = sql.NullString{String: t.TempDir(), Valid: true}
	wait := &sync.WaitGroup{}
	testScanner.scanner.wait = wait

	started := testScanner.scanner.Start()
	if started.Status != StartStarted || started.Directory != testScanner.moviesDir.String {
		t.Fatalf("configured Start result = %+v, want started for %q", started, testScanner.moviesDir.String)
	}

	alreadyRunning := testScanner.scanner.Start()
	if alreadyRunning.Status != StartAlreadyRunning {
		t.Fatalf("concurrent Start status = %v, want %v", alreadyRunning.Status, StartAlreadyRunning)
	}

	wait.Wait()
	restarted := testScanner.scanner.Start()
	if restarted.Status != StartStarted {
		t.Fatalf("Start after scan completion = %v, want %v", restarted.Status, StartStarted)
	}
	wait.Wait()
}

func TestNewDefaultsOptionalDependencies(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Bare.Deps.2024.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	// Only the required dependencies. Everything else must be defaulted by New,
	// or the scan below panics on a nil mutex, wait group, or context.
	bare := New(Dependencies{
		DB:      testScanner.db,
		Queries: testScanner.queries,
		Logger:  &capturedLogger{},
		Ffprobe: &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")},
	})

	if bare.scanContext == nil || bare.wait == nil || bare.scannerDBMu == nil {
		t.Fatal("New left an optional dependency nil")
	}
	if bare.currentMoviesDirectory == nil || bare.invalidateCommittedMovie == nil {
		t.Fatal("New left an optional callback nil")
	}

	directory := bare.currentMoviesDirectory()
	if directory.Valid {
		t.Fatalf("default movies directory = %+v, want unset", directory)
	}

	result := bare.Start()
	if result.Status != StartNotConfigured {
		t.Fatalf("Start() status = %d, want StartNotConfigured", result.Status)
	}

	// Drive the write path directly, since Start short-circuits without a
	// configured directory: this is what would panic on a nil ScannerDBMu.
	resolved, err := bare.resolveMovieFile(context.Background(), scanner.ScanFile{Path: path, Ext: "mkv", Size: 5})
	if err != nil {
		t.Fatalf("resolve movie with bare dependencies: %v", err)
	}

	err = bare.persistResolvedMovie(context.Background(), newMovieScanContext(nil), resolved)
	if err != nil {
		t.Fatalf("persist movie with bare dependencies: %v", err)
	}

	if got := countScannerRows(t, testScanner.db, "SELECT COUNT(*) FROM movies WHERE file_path = ?", path); got != 1 {
		t.Fatalf("expected the movie to persist, got %d rows", got)
	}
}

func TestProcessMoviesBatchSkipsUnchangedWithoutFfprobe(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Unchanged.Movie.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	ffprobeStub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	testScanner.scanner.ffprobe = ffprobeStub

	scan := newMovieScanContext(map[string]int64{path: 5})
	scanned, skipped, errCount := testScanner.scanner.processMoviesBatch(context.Background(), scan, []scanner.ScanFile{
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
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Audio.Only.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{
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
	testScanner.scanner.tmdb = &stubMovieScannerTmdb{searchErr: errors.New("tmdb unavailable")}

	scanned, skipped, errCount := testScanner.scanner.processMoviesBatch(context.Background(), newMovieScanContext(nil), []scanner.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})

	if scanned != 0 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/0/1", scanned, skipped, errCount)
	}
	if got := countScannerRows(t, testScanner.db, "SELECT COUNT(*) FROM movies WHERE file_path = ?", path); got != 0 {
		t.Fatalf("expected invalid movie transaction to roll back, got %d movie rows", got)
	}
}

func TestProcessMoviesBatchRollbackLeavesScanCachesUnpolluted(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Audio.Only.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	// No video stream, so persistResolvedMovieTx fails after the genre and
	// artist caches were already written inside the transaction.
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{
		result: &ffprobe.FfprobeResult{
			Format: ffprobe.Format{Duration: "120"},
			Streams: []ffprobe.Stream{
				{Index: 0, CodecName: "aac", CodecType: "audio", Channels: 2},
			},
		},
	}

	details := tmdbMovieFromJSON(t, `{
		"id": 4242,
		"title": "Audio Only",
		"release_date": "2020-01-01",
		"genres": [{"id": 18, "name": "Drama"}],
		"credits": {"cast": [{"id": 77, "name": "Rolled Back Actor", "character": "Self", "order": 0}]}
	}`)
	testScanner.scanner.tmdb = &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{{TmdbID: 4242, Title: "Audio Only", ReleaseDate: "2020-01-01"}},
		detailMovies:  map[int]tmdb.TmdbMovie{4242: details},
	}

	scan := newMovieScanContext(nil)
	scanned, skipped, errCount := testScanner.scanner.processMoviesBatch(context.Background(), scan, []scanner.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})

	if scanned != 0 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/0/1", scanned, skipped, errCount)
	}

	_, cachedArtist := scan.artistIDs.Get(77)
	if cachedArtist {
		t.Fatal("rolled-back transaction published an artist id into the scan cache")
	}

	_, cachedGenre := scan.genreIDs.Get(scanner.NormalizedScanCacheKey("Drama", "movie"))
	if cachedGenre {
		t.Fatal("rolled-back transaction published a genre id into the scan cache")
	}

	_, indexed := scan.movieIndex[filepath.Clean(path)]
	if indexed {
		t.Fatal("rolled-back transaction recorded the movie as scanned in the scan index")
	}
}

func TestRunMovieScanPreservesMissingMovieRows(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	missingPath := filepath.Join(moviesDir, "Missing.Movie.1999.mkv")
	movie, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Missing Movie",
		FilePath:  missingPath,
		FileName:  filepath.Base(missingPath),
		Size:      7,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert missing movie: %v", err)
	}

	testScanner.moviesDir = sql.NullString{String: moviesDir, Valid: true}
	testScanner.scanner.runMovieScan()

	_, err = testScanner.queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("expected missing movie row to be preserved: %v", err)
	}
}

func TestRunMovieScan_AcceptsConfiguredVideoExtensions(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

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

	ffprobeStub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	testScanner.scanner.ffprobe = ffprobeStub
	testScanner.moviesDir = sql.NullString{String: moviesDir, Valid: true}

	testScanner.scanner.runMovieScan()

	if ffprobeStub.calls != len(files) {
		t.Fatalf("ffprobe calls = %d, want %d", ffprobeStub.calls, len(files))
	}

	for _, file := range files {
		var container string
		err := testScanner.db.QueryRowContext(context.Background(), `
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

func TestRunMovieScanWalksVideoFilesAndLogsOnlyFinalResults(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	moviesDir := t.TempDir()
	for i := 0; i < scanner.BatchSize+1; i++ {
		path := filepath.Join(moviesDir, "Movie."+strconv.Itoa(i)+".2020.mkv")
		if err := os.WriteFile(path, []byte("movie"), 0o644); err != nil {
			t.Fatalf("write movie %d: %v", i, err)
		}
	}
	if err := os.WriteFile(filepath.Join(moviesDir, "not-a-movie.txt"), []byte("text"), 0o644); err != nil {
		t.Fatalf("write non-video file: %v", err)
	}

	logger := &capturedLogger{}
	testScanner.scanner.logger = logger
	ffprobeStub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	testScanner.scanner.ffprobe = ffprobeStub
	testScanner.moviesDir = sql.NullString{String: moviesDir, Valid: true}

	testScanner.scanner.runMovieScan()

	if ffprobeStub.calls != scanner.BatchSize+1 {
		t.Fatalf("ffprobe calls = %d, want %d video files only", ffprobeStub.calls, scanner.BatchSize+1)
	}

	foundCompletion := false
	wantCompletion := "movies scanner completed: " + strconv.Itoa(scanner.BatchSize+1) + " scanned, 0 skipped, 0 errors"
	for _, entry := range logger.infoEntries {
		if strings.Contains(entry.msg, "movies scanner batch processed") {
			t.Fatalf("unexpected per-batch log entry: %q", entry.msg)
		}
		if strings.Contains(entry.msg, wantCompletion) {
			foundCompletion = true
		}
	}
	if !foundCompletion {
		t.Fatalf("missing final completion log; info entries = %+v", logger.infoEntries)
	}
}
