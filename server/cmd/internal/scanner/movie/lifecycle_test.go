package movie

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"

	_ "github.com/mattn/go-sqlite3"
)

func TestStartStatusesAndGuardRelease(t *testing.T) {
	testScanner := setupMovieScanner(t)

	notConfigured := testScanner.scanner.Start()
	if notConfigured.Status != scanner.StartNotConfigured {
		t.Fatalf("unconfigured Start status = %v, want %v", notConfigured.Status, scanner.StartNotConfigured)
	}

	testScanner.moviesDir = sql.NullString{String: t.TempDir(), Valid: true}
	path := filepath.Join(testScanner.moviesDir.String, "Lifecycle.2024.mkv")
	scannertest.WriteFile(t, path, "movie")
	wait := &sync.WaitGroup{}
	testScanner.scanner.launcher.Wait = wait
	ctx, cancel := context.WithCancel(context.Background())
	testScanner.scanner.scanContext = ctx
	probe := scannertest.NewGateProbe(movieScannerMetadataFixture("120"))
	testScanner.scanner.ffprobe = probe
	defer func() {
		cancel()
		probe.Release()
		scannertest.WaitForGroup(t, wait, 5*time.Second, "movie scan stop")
	}()

	started := testScanner.scanner.Start()
	if started.Status != scanner.StartStarted || started.Directory != testScanner.moviesDir.String {
		t.Fatalf("configured Start result = %+v, want started for %q", started, testScanner.moviesDir.String)
	}

	probe.WaitEntered(t, 5*time.Second, "movie scan reaching probing")
	alreadyRunning := testScanner.scanner.Start()
	if alreadyRunning.Status != scanner.StartAlreadyRunning {
		t.Fatalf("concurrent Start status = %v, want %v", alreadyRunning.Status, scanner.StartAlreadyRunning)
	}

	probe.Release()
	scannertest.WaitForGroup(t, wait, 5*time.Second, "movie scan stop")
	restarted := testScanner.scanner.Start()
	if restarted.Status != scanner.StartStarted {
		t.Fatalf("Start after scan completion = %v, want %v", restarted.Status, scanner.StartStarted)
	}
	scannertest.WaitForGroup(t, wait, 5*time.Second, "movie scan stop")
}

func TestNewDefaultsOptionalDependencies(t *testing.T) {
	testScanner := setupMovieScanner(t)

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Bare.Deps.2024.mkv")
	scannertest.WriteFile(t, path, "movie")

	// Only the required dependencies. Everything else must be defaulted by New,
	// or the scan below panics on a nil mutex, wait group, or context.
	bare := New(Dependencies{
		DB:      testScanner.db,
		Queries: testScanner.queries,
		Logger:  &scannertest.Logger{},
		Ffprobe: &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")},
	})

	if bare.scanContext == nil || bare.launcher.Wait == nil || bare.tx.Mu == nil {
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
	if result.Status != scanner.StartNotConfigured {
		t.Fatalf("Start() status = %d, want scanner.StartNotConfigured", result.Status)
	}

	// Drive the write path directly, since Start short-circuits without a
	// configured directory: this is what would panic on a nil ScannerDBMu.
	resolved, err := bare.resolveMovieFile(context.Background(), scanner.ScanFile{Path: path, Ext: "mkv", Size: 5})
	if err != nil {
		t.Fatalf("resolve movie with bare dependencies: %v", err)
	}

	resolved.inspection, err = scanner.InspectFileMetadata(context.Background(), path, nil, testScanner.scanner.now)
	if err != nil {
		t.Fatal(err)
	}
	defer resolved.inspection.Close()
	err = bare.persistLocalMovie(context.Background(), newMovieScanContext(nil), resolved.localMovie)
	if err != nil {
		t.Fatalf("persist movie with bare dependencies: %v", err)
	}

	got := scannertest.CountRows(t, testScanner.db, "SELECT COUNT(*) FROM movies WHERE file_path = ?", path)
	if got != 1 {
		t.Fatalf("expected the movie to persist, got %d rows", got)
	}
}

func TestProcessMoviesBatchRollbackLeavesScanIndexUnpolluted(t *testing.T) {
	testScanner := setupMovieScanner(t)

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Audio.Only.2020.mkv")
	scannertest.WriteFile(t, path, "movie")

	// No video stream, so persistLocalMovie fails inside its transaction.
	// The scan index is only published after a commit, so the rollback must
	// leave the file unrecorded and eligible for the next scan.
	testScanner.scanner.ffprobe = &scannertest.CountingProbe{
		Default: &ffprobe.FfprobeResult{
			Format: ffprobe.Format{Duration: "120"},
			Streams: []ffprobe.Stream{
				{Index: 0, CodecName: "aac", CodecType: "audio", Channels: 2},
			},
		},
	}

	scan := newMovieScanContext(nil)
	scanned, skipped, errCount, _ := testScanner.scanner.processMoviesBatch(context.Background(), scan, []scanner.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})

	if scanned != 0 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/0/1", scanned, skipped, errCount)
	}

	_, indexed := scan.movieIndex[filepath.Clean(path)]
	if indexed {
		t.Fatal("rolled-back transaction recorded the movie as scanned in the scan index")
	}
}

func TestRunMovieScan_AcceptsConfiguredVideoExtensions(t *testing.T) {
	testScanner := setupMovieScanner(t)

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
		scannertest.WriteFile(t, file.path, "movie")
	}

	ffprobeStub := &scannertest.CountingProbe{Default: movieScannerMetadataFixture("120")}
	testScanner.scanner.ffprobe = ffprobeStub
	testScanner.moviesDir = sql.NullString{String: moviesDir, Valid: true}

	testScanner.scanner.scan(testScanner.moviesDir.String)

	if ffprobeStub.Calls() != len(files) {
		t.Fatalf("ffprobe calls = %d, want %d", ffprobeStub.Calls(), len(files))
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
