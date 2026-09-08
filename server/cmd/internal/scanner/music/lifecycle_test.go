package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/sqlc"

	"github.com/mattn/go-sqlite3"
	spotifylib "github.com/zmb3/spotify/v2"
)

func TestRunMusicScanDoesNotClearSpotifyRuntimeCache(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "Test Track.m4a")
	err := os.WriteFile(trackPath, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("write track file: %v", err)
	}

	spotifyStub := &musicScannerSpotifyStub{}
	app.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	app.spotify = spotifyStub
	app.currentMusicDirectory = func() sql.NullString {
		return sql.NullString{String: musicDir, Valid: true}
	}

	app.runMusicScan(app.currentMusicDirectory().String)

	if spotifyStub.clearCalls != 0 {
		t.Fatalf("spotify cache clear calls = %d, want 0", spotifyStub.clearCalls)
	}
}

func TestRunMusicScanLogsCancellationFromFinalPartialBatch(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "Canceled Track.m4a")
	writeMusicScannerTestFile(t, trackPath, "test")

	ctx, cancel := context.WithCancel(context.Background())
	ffprobeStub := &cancelingMusicScannerFfprobe{cancel: cancel}
	logger := &capturedLogger{}
	app.scanContext = ctx
	app.ffprobe = ffprobeStub
	app.logger = logger
	app.currentMusicDirectory = func() sql.NullString {
		return sql.NullString{String: musicDir, Valid: true}
	}

	runMusicScanForTest(t, app)

	if ffprobeStub.calls != 1 {
		t.Fatalf("audio metadata calls = %d, want 1", ffprobeStub.calls)
	}

	foundCancellation := false
	for _, entry := range logger.infoEntries {
		if entry.msg == "music library scan interrupted" {
			foundCancellation = true
		}
		if strings.HasPrefix(entry.msg, "music scanner completed:") {
			t.Fatalf("canceled scan logged completion: %q", entry.msg)
		}
	}
	if !foundCancellation {
		t.Fatalf("missing cancellation log; info entries = %+v", logger.infoEntries)
	}
}

func TestRunMusicScanWalksAudioFilesAndSkipsUnchangedFiles(t *testing.T) {
	app := setupMusicScanner(t)
	defer app.db.Close()

	musicDir := t.TempDir()
	m4aPath := filepath.Join(musicDir, "Album", "Track One.m4a")
	mp3Path := filepath.Join(musicDir, "Album", "Track Two.MP3")
	flacPath := filepath.Join(musicDir, "Nested", "Track Three.flac")
	ignoredPath := filepath.Join(musicDir, "Album", "cover.jpg")

	writeMusicScannerTestFile(t, m4aPath, "m4a")
	writeMusicScannerTestFile(t, mp3Path, "mp3")
	writeMusicScannerTestFile(t, flacPath, "flac")
	writeMusicScannerTestFile(t, ignoredPath, "jpg")

	ffprobeStub := newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		m4aPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Track One",
			Artist: "Walk Artist",
			Album:  "Walk Album",
			Track:  "1/3",
		}),
		mp3Path: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Track Two",
			Artist: "Walk Artist",
			Album:  "Walk Album",
			Track:  "2/3",
		}),
		flacPath: testMusicMetadataWithTags(ffprobe.FormatTags{
			Title:  "Track Three",
			Artist: "Walk Artist",
			Album:  "Walk Album",
			Track:  "3/3",
		}),
	})
	app.ffprobe = ffprobeStub
	app.currentMusicDirectory = func() sql.NullString {
		return sql.NullString{String: musicDir, Valid: true}
	}

	runMusicScanForTest(t, app)

	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM tracks"); got != 3 {
		t.Fatalf("track count after first scan = %d, want 3", got)
	}
	if got := countScannerRows(t, app.db, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", ignoredPath); got != 0 {
		t.Fatalf("ignored file track count = %d, want 0", got)
	}
	if ffprobeStub.totalAudioCalls() != 3 {
		t.Fatalf("audio metadata calls = %d, want 3", ffprobeStub.totalAudioCalls())
	}
	if ffprobeStub.totalMetadataCalls() != 0 {
		t.Fatalf("generic metadata calls = %d, want 0", ffprobeStub.totalMetadataCalls())
	}

	runMusicScanForTest(t, app)

	if ffprobeStub.totalAudioCalls() != 3 {
		t.Fatalf("audio metadata calls after unchanged rescan = %d, want 3", ffprobeStub.totalAudioCalls())
	}

	newSize := writeMusicScannerTestFile(t, mp3Path, "mp3 changed")
	ffprobeStub.results[mp3Path] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Track Two Updated",
		Artist: "Walk Artist",
		Album:  "Walk Album",
		Track:  "2/3",
	})

	runMusicScanForTest(t, app)

	if ffprobeStub.totalAudioCalls() != 4 {
		t.Fatalf("audio metadata calls after changed rescan = %d, want 4", ffprobeStub.totalAudioCalls())
	}
	if ffprobeStub.audioCalls[mp3Path] != 2 {
		t.Fatalf("changed file audio calls = %d, want 2", ffprobeStub.audioCalls[mp3Path])
	}

	var title string
	var size int64
	err := app.db.QueryRow("SELECT title, size FROM tracks WHERE file_path = ?", mp3Path).Scan(&title, &size)
	if err != nil {
		t.Fatalf("get updated track: %v", err)
	}
	if title != "Track Two Updated" || size != newSize {
		t.Fatalf("updated track title/size = %q/%d, want %q/%d", title, size, "Track Two Updated", newSize)
	}
}

func TestStartRejectsUnconfiguredDirectories(t *testing.T) {
	for _, directory := range []sql.NullString{{}, {String: "/music"}, {Valid: true}} {
		s := New(Dependencies{
			CurrentMusicDirectory: func() sql.NullString { return directory },
		})
		result := s.Start()
		if result.Status != StartNotConfigured {
			t.Fatalf("Start(%+v) = %+v, want not configured", directory, result)
		}
	}
}

func TestStartReleasesGuardAndTracksShutdown(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	directory := t.TempDir()
	s.currentMusicDirectory = func() sql.NullString {
		return sql.NullString{String: directory, Valid: true}
	}
	s.wait = &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.scanContext = ctx

	for i := 0; i < 2; i++ {
		result := s.Start()
		if result.Status != StartStarted {
			t.Fatalf("Start %d = %+v, want started", i, result)
		}
		s.wait.Wait()
	}
}

func TestScannerContextFallbackAndOptionalWait(t *testing.T) {
	s := New(Dependencies{})
	if s.wait == nil {
		t.Fatal("missing wait group was not defaulted")
	}
	ctx := s.scanContext
	if ctx != context.Background() {
		t.Fatal("missing scan context did not fall back to background")
	}
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = shutdownCtx
	if s.scanContext != shutdownCtx {
		t.Fatal("scanner did not use shutdown context")
	}
}

type callbackMusicProbe struct {
	noKeyframeProbe
	audio func(context.Context, string) (*ffprobe.FfprobeResult, error)
}

func (p *callbackMusicProbe) GetMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.audio(ctx, path)
}

func (p *callbackMusicProbe) GetAudioMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.audio(ctx, path)
}

func waitForMusicScan(t *testing.T, s *Scanner) {
	t.Helper()
	done := make(chan struct{})
	go func() { s.wait.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not stop")
	}
}

func TestStartCapturesDirectoryAndUsesInstanceGuard(t *testing.T) {
	first := setupMusicScanner(t)
	defer first.db.Close()
	directory := t.TempDir()
	path := filepath.Join(directory, "track.m4a")
	writeMusicScannerTestFile(t, path, "audio")
	calls := atomic.Int32{}
	first.currentMusicDirectory = func() sql.NullString {
		if calls.Add(1) == 1 {
			return sql.NullString{String: directory, Valid: true}
		}
		return sql.NullString{String: "/changed/music", Valid: true}
	}
	entered := make(chan string, 1)
	release := make(chan struct{})
	first.ffprobe = &callbackMusicProbe{audio: func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
		entered <- path
		<-release
		return testMusicMetadata(), nil
	}}
	defer func() { close(release); waitForMusicScan(t, first) }()
	result := first.Start()
	if result.Status != StartStarted || result.Directory != directory {
		t.Fatalf("first start: %+v", result)
	}
	select {
	case got := <-entered:
		if got != path || calls.Load() != 1 {
			t.Fatalf("probed %q, directory calls=%d", got, calls.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not probe captured directory")
	}
	result = first.Start()
	if result.Status != StartAlreadyRunning {
		t.Fatalf("overlapping start: %+v", result)
	}
	second := setupMusicScanner(t)
	defer second.db.Close()
	second.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
	second.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
	result = second.Start()
	if result.Status != StartStarted {
		t.Fatalf("independent scanner start: %+v", result)
	}
	waitForMusicScan(t, second)
	count := countScannerRows(t, second.db, "SELECT count(*) FROM tracks")
	if count != 1 {
		t.Fatalf("independent scanner wrote %d tracks", count)
	}
}

func TestStartReleasesGuardAfterFailures(t *testing.T) {
	for _, phase := range []string{"index", "walk", "probe"} {
		t.Run(phase, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			directory := t.TempDir()
			s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
			switch phase {
			case "index":
				_, err := s.db.Exec("DROP TABLE tracks")
				if err != nil {
					t.Fatal(err)
				}
			case "walk":
				directory = filepath.Join(directory, "absent")
			case "probe":
				writeMusicScannerTestFile(t, filepath.Join(directory, "fail.m4a"), "audio")
				s.ffprobe = &callbackMusicProbe{audio: func(context.Context, string) (*ffprobe.FfprobeResult, error) { return nil, errors.New("probe failure") }}
			}
			for i := 0; i < 2; i++ {
				result := s.Start()
				if result.Status != StartStarted {
					t.Fatalf("start %d: %+v", i, result)
				}
				waitForMusicScan(t, s)
			}
		})
	}
}

func assertMusicInterrupted(t *testing.T, s *Scanner) {
	t.Helper()
	logs := s.logger.(*capturedLogger)
	interrupted := 0
	for _, entry := range logs.infoEntries {
		if entry.msg == "music library scan interrupted" {
			interrupted++
		}
		if strings.Contains(entry.msg, "completed:") {
			t.Errorf("interrupted scan logged completion: %s", entry.msg)
		}
	}
	if interrupted != 1 || len(logs.warnEntries) != 0 || len(logs.errorEntries) != 0 {
		t.Fatalf("interrupted=%d warnings=%+v errors=%+v", interrupted, logs.warnEntries, logs.errorEntries)
	}
}

func TestShutdownDuringIndexLoading(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	directory := t.TempDir()
	s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	result := s.Start()
	if result.Status != StartStarted {
		t.Fatal(result)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s.db.Stats().WaitCount == 0 {
		if time.Now().After(deadline) {
			t.Fatal("index query did not wait for database")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	waitForMusicScan(t, s)
	assertMusicInterrupted(t, s)
}

func TestShutdownDuringProbeBatches(t *testing.T) {
	for _, count := range []int{1, scanner.BatchSize + 1} {
		for _, expiry := range []bool{false, true} {
			t.Run(fmt.Sprintf("files=%d/expiry=%v", count, expiry), func(t *testing.T) {
				s := setupMusicScanner(t)
				defer s.db.Close()
				directory := t.TempDir()
				for i := 0; i < count; i++ {
					writeMusicScannerTestFile(t, filepath.Join(directory, fmt.Sprintf("%03d.m4a", i)), "audio")
				}
				ctx, cancel := context.WithCancel(context.Background())
				if expiry {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				}
				defer cancel()
				s.scanContext = ctx
				s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
				calls := 0
				s.ffprobe = &callbackMusicProbe{audio: func(ctx context.Context, _ string) (*ffprobe.FfprobeResult, error) {
					calls++
					if !expiry {
						cancel()
					}
					<-ctx.Done()
					return nil, ctx.Err()
				}}
				result := s.Start()
				if result.Status != StartStarted {
					t.Fatal(result)
				}
				waitForMusicScan(t, s)
				assertMusicInterrupted(t, s)
				if calls != 1 {
					t.Fatalf("probe calls=%d", calls)
				}
				rows := countScannerRows(t, s.db, "SELECT count(*) FROM tracks")
				if rows != 0 {
					t.Fatalf("canceled scan wrote %d tracks", rows)
				}
			})
		}
	}
}

type cancelingMusicSpotify struct {
	musicScannerSpotifyStub
	cancel context.CancelFunc
	phase  string
}

func (s *cancelingMusicSpotify) SearchArtistByName(context.Context, string) (*spotifylib.FullArtist, error) {
	s.artistCalls++
	if s.phase == "artist" {
		s.cancel()
		return nil, context.Canceled
	}
	return nil, nil
}

func (s *cancelingMusicSpotify) SearchAndGetAlbumDetails(context.Context, string, string) (*spotifylib.FullAlbum, error) {
	s.albumCalls++
	s.cancel()
	return nil, context.Canceled
}

func TestShutdownDuringSpotifyResolution(t *testing.T) {
	musicDir := t.TempDir()
	for _, phase := range []string{"artist", "album"} {
		t.Run(phase, func(t *testing.T) {
			s := setupMusicScanner(t)
			defer s.db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			spotify := &cancelingMusicSpotify{cancel: cancel, phase: phase}
			s.spotify = spotify
			s.ffprobe = &countingMusicScannerFfprobe{result: testMusicMetadata()}
			scan := newMusicScanContext(nil)
			scanned, _, failures := s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{{Path: musicDir + "/cancel.m4a", Ext: "m4a", Size: 1}, {Path: musicDir + "/later.m4a", Ext: "m4a", Size: 1}})
			if scanned != 0 || failures != 0 || spotify.artistCalls != 1 {
				t.Fatalf("scanned=%d errors=%d Spotify=%+v", scanned, failures, spotify)
			}
			if len(scan.spotifyArtistMisses) != 0 || len(scan.spotifyAlbumMisses) != 0 {
				t.Fatal("cancellation cached as Spotify failure")
			}
			for _, table := range []string{"tracks", "musicians", "albums", "music_spotify_matches"} {
				count := countScannerRows(t, s.db, "SELECT count(*) FROM "+table)
				if count != 0 {
					t.Fatalf("canceled resolution wrote to %s", table)
				}
			}
		})
	}
}

func TestShutdownWaitingForPersistence(t *testing.T) {
	s := setupMusicScanner(t)
	defer s.db.Close()
	directory := t.TempDir()
	writeMusicScannerTestFile(t, filepath.Join(directory, "track.m4a"), "audio")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
	entered := make(chan struct{})
	s.ffprobe = &callbackMusicProbe{audio: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		close(entered)
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track"}), nil
	}}
	s.scannerDBMu.Lock()
	result := s.Start()
	if result.Status != StartStarted {
		s.scannerDBMu.Unlock()
		t.Fatal(result)
	}
	<-entered
	cancel()
	s.scannerDBMu.Unlock()
	waitForMusicScan(t, s)
	assertMusicInterrupted(t, s)
	rows := countScannerRows(t, s.db, "SELECT count(*) FROM tracks")
	if rows != 0 {
		t.Fatalf("canceled persistence wrote %d tracks", rows)
	}
}

func TestShutdownDuringTrackTransaction(t *testing.T) {
	musicDir := t.TempDir()
	// A file database survives database/sql discarding a canceled connection.
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "music.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(sqlc.Schema)
	if err != nil {
		t.Fatal(err)
	}
	queries, err := database.Prepare(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer queries.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Raw(func(raw any) error {
		return raw.(*sqlite3.SQLiteConn).RegisterFunc("interrupt_music", func() int { cancel(); return 0 }, false)
	})
	conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("CREATE TRIGGER interrupt_track AFTER INSERT ON tracks BEGIN SELECT interrupt_music(); END")
	if err != nil {
		t.Fatal(err)
	}
	invalidations := 0
	s := New(Dependencies{Now: func() time.Time { return time.Now().Add(2 * time.Minute) }, DB: db, Queries: queries, Logger: &capturedLogger{}, Ffprobe: &countingMusicScannerFfprobe{result: testMusicMetadata()}, InvalidateCommittedTrack: func(int64) { invalidations++ }})
	scan := newMusicScanContext(nil)
	scanned, _, failures := s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{{Path: musicDir + "/interrupt.m4a", Ext: "m4a", Size: 1}})
	if scanned != 0 || failures != 0 || invalidations != 0 {
		t.Fatalf("scanned=%d failures=%d invalidations=%d", scanned, failures, invalidations)
	}
	if !reflect.DeepEqual(scan, newMusicScanContext(nil)) {
		t.Fatal("canceled transaction published caches")
	}
	for _, table := range []string{"tracks", "musicians", "albums", "musician_albums"} {
		count := countScannerRows(t, db, "SELECT count(*) FROM "+table)
		if count != 0 {
			t.Fatalf("canceled transaction left %d rows in %s", count, table)
		}
	}
}
