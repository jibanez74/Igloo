package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"

	"github.com/mattn/go-sqlite3"
	spotifylib "github.com/zmb3/spotify/v2"
)

func TestRunMusicScanLogsCancellationFromFinalPartialBatch(t *testing.T) {
	app := setupMusicScanner(t)

	musicDir := t.TempDir()
	trackPath := filepath.Join(musicDir, "Canceled Track.m4a")
	scannertest.WriteFile(t, trackPath, "test")

	ctx, cancel := context.WithCancel(context.Background())
	ffprobeStub := &scannertest.CountingProbe{Hook: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		cancel()
		return nil, context.Canceled
	}}
	logger := &scannertest.Logger{}
	app.scanContext = ctx
	app.ffprobe = ffprobeStub
	app.logger = logger
	app.currentMusicDirectory = func() sql.NullString {
		return sql.NullString{String: musicDir, Valid: true}
	}

	app.scan(app.currentMusicDirectory().String)

	if ffprobeStub.Calls() != 1 {
		t.Fatalf("audio metadata calls = %d, want 1", ffprobeStub.Calls())
	}

	assertMusicInterrupted(t, app)
	if scannertest.CountRows(t, app.tx.DB, "SELECT count(*) FROM tracks") != 0 {
		t.Fatal("canceled scan persisted the track")
	}
}

func TestRunMusicScanWalksAudioFilesAndSkipsUnchangedFiles(t *testing.T) {
	app := setupMusicScanner(t)

	musicDir := t.TempDir()
	m4aPath := filepath.Join(musicDir, "Album", "Track One.m4a")
	mp3Path := filepath.Join(musicDir, "Album", "Track Two.MP3")
	flacPath := filepath.Join(musicDir, "Nested", "Track Three.flac")
	ignoredPath := filepath.Join(musicDir, "Album", "cover.jpg")

	scannertest.WriteFile(t, m4aPath, "m4a")
	scannertest.WriteFile(t, mp3Path, "mp3")
	scannertest.WriteFile(t, flacPath, "flac")
	scannertest.WriteFile(t, ignoredPath, "jpg")

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

	app.scan(app.currentMusicDirectory().String)

	got := scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM tracks")
	if got != 3 {
		t.Fatalf("track count after first scan = %d, want 3", got)
	}
	got = scannertest.CountRows(t, app.tx.DB, "SELECT COUNT(*) FROM tracks WHERE file_path = ?", ignoredPath)
	if got != 0 {
		t.Fatalf("ignored file track count = %d, want 0", got)
	}
	if ffprobeStub.totalAudioCalls() != 3 {
		t.Fatalf("audio metadata calls = %d, want 3", ffprobeStub.totalAudioCalls())
	}
	if ffprobeStub.totalMetadataCalls() != 0 {
		t.Fatalf("generic metadata calls = %d, want 0", ffprobeStub.totalMetadataCalls())
	}

	app.scan(app.currentMusicDirectory().String)

	if ffprobeStub.totalAudioCalls() != 3 {
		t.Fatalf("audio metadata calls after unchanged rescan = %d, want 3", ffprobeStub.totalAudioCalls())
	}

	scannertest.WriteFile(t, mp3Path, "mp3 changed")
	newSize := int64(len("mp3 changed"))
	ffprobeStub.results[mp3Path] = testMusicMetadataWithTags(ffprobe.FormatTags{
		Title:  "Track Two Updated",
		Artist: "Walk Artist",
		Album:  "Walk Album",
		Track:  "2/3",
	})

	app.scan(app.currentMusicDirectory().String)

	if ffprobeStub.totalAudioCalls() != 4 {
		t.Fatalf("audio metadata calls after changed rescan = %d, want 4", ffprobeStub.totalAudioCalls())
	}
	if ffprobeStub.audioCalls[mp3Path] != 2 {
		t.Fatalf("changed file audio calls = %d, want 2", ffprobeStub.audioCalls[mp3Path])
	}

	var title string
	var size int64
	err := app.tx.DB.QueryRow("SELECT title, size FROM tracks WHERE file_path = ?", mp3Path).Scan(&title, &size)
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
		if result.Status != scanner.StartNotConfigured {
			t.Fatalf("Start(%+v) = %+v, want not configured", directory, result)
		}
	}
}

func TestStartReleasesGuardAndTracksShutdown(t *testing.T) {
	s := setupMusicScanner(t)
	directory := t.TempDir()
	s.currentMusicDirectory = func() sql.NullString {
		return sql.NullString{String: directory, Valid: true}
	}
	s.launcher.Wait = &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.scanContext = ctx

	for i := 0; i < 2; i++ {
		result := s.Start()
		if result.Status != scanner.StartStarted {
			t.Fatalf("Start %d = %+v, want started", i, result)
		}
		scannertest.WaitForGroup(t, s.launcher.Wait, 5*time.Second, "music scan")
	}
}

func TestStartCapturesDirectoryAndUsesInstanceGuard(t *testing.T) {
	first := setupMusicScanner(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "track.m4a")
	scannertest.WriteFile(t, path, "audio")
	calls := atomic.Int32{}
	first.currentMusicDirectory = func() sql.NullString {
		if calls.Add(1) == 1 {
			return sql.NullString{String: directory, Valid: true}
		}
		return sql.NullString{String: "/changed/music", Valid: true}
	}
	entered := make(chan string, 1)
	release := make(chan struct{})
	first.ffprobe = &scannertest.Probe{Callback: func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
		entered <- path
		<-release
		return testMusicMetadata(), nil
	}}
	defer func() { close(release); scannertest.WaitForGroup(t, first.launcher.Wait, 5*time.Second, "music scan") }()
	result := first.Start()
	if result.Status != scanner.StartStarted || result.Directory != directory {
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
	if result.Status != scanner.StartAlreadyRunning {
		t.Fatalf("overlapping start: %+v", result)
	}
	second := setupMusicScanner(t)
	second.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
	second.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	result = second.Start()
	if result.Status != scanner.StartStarted {
		t.Fatalf("independent scanner start: %+v", result)
	}
	scannertest.WaitForGroup(t, second.launcher.Wait, 5*time.Second, "music scan")
	count := scannertest.CountRows(t, second.tx.DB, "SELECT count(*) FROM tracks")
	if count != 1 {
		t.Fatalf("independent scanner wrote %d tracks", count)
	}
}

func TestStartReleasesGuardAfterFailures(t *testing.T) {
	for _, phase := range []string{"index", "walk", "probe"} {
		t.Run(phase, func(t *testing.T) {
			s := setupMusicScanner(t)
			directory := t.TempDir()
			s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
			switch phase {
			case "index":
				_, err := s.tx.DB.Exec("DROP TABLE tracks")
				if err != nil {
					t.Fatal(err)
				}
			case "walk":
				directory = filepath.Join(directory, "absent")
			case "probe":
				scannertest.WriteFile(t, filepath.Join(directory, "fail.m4a"), "audio")
				s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) { return nil, errors.New("probe failure") }}
			}
			for i := 0; i < 2; i++ {
				result := s.Start()
				if result.Status != scanner.StartStarted {
					t.Fatalf("start %d: %+v", i, result)
				}
				scannertest.WaitForGroup(t, s.launcher.Wait, 5*time.Second, "music scan")
			}
		})
	}
}

func assertMusicInterrupted(t *testing.T, s *Scanner) {
	t.Helper()
	logs := s.logger.(*scannertest.Logger)
	interrupted := 0
	for _, entry := range logs.WarnEntries {
		if entry.Msg == "music library scan interrupted" {
			interrupted++
		}
	}
	if s.Status().State != scanner.StateCanceled {
		t.Errorf("interrupted scan reported state %q", s.Status().State)
	}
	if interrupted != 1 || len(logs.WarnEntries) != interrupted || len(logs.ErrorEntries) != 0 {
		t.Fatalf("interrupted=%d warnings=%+v errors=%+v", interrupted, logs.WarnEntries, logs.ErrorEntries)
	}
}

func TestShutdownDuringIndexLoading(t *testing.T) {
	s := setupMusicScanner(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	directory := t.TempDir()
	s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
	conn, err := s.tx.DB.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	result := s.Start()
	if result.Status != scanner.StartStarted {
		t.Fatal(result)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s.tx.DB.Stats().WaitCount == 0 {
		if time.Now().After(deadline) {
			t.Fatal("index query did not wait for database")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	scannertest.WaitForGroup(t, s.launcher.Wait, 5*time.Second, "music scan")
	assertMusicInterrupted(t, s)
}

// A cancellation in the final partial batch is covered by
// TestRunMusicScanLogsCancellationFromFinalPartialBatch; this drives it
// through a full batch, by explicit cancel and by context expiry.
func TestShutdownDuringProbeBatches(t *testing.T) {
	const count = scanner.BatchSize + 1
	for _, expiry := range []bool{false, true} {
		t.Run(fmt.Sprintf("expiry=%v", expiry), func(t *testing.T) {
			s := setupMusicScanner(t)
			directory := t.TempDir()
			for i := 0; i < count; i++ {
				scannertest.WriteFile(t, filepath.Join(directory, fmt.Sprintf("%03d.m4a", i)), "audio")
			}
			// The first probe ends the scan context either way; an expiring
			// context stands in for a deadline so it lands mid-probe instead
			// of whenever a timer fires relative to the scan's progress.
			var ctx context.Context
			var end func()
			if expiry {
				expiring := newExpiringContext()
				ctx, end = expiring, expiring.expire
			} else {
				ctx, end = context.WithCancel(context.Background())
			}
			defer end()
			s.scanContext = ctx
			s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
			calls := 0
			s.ffprobe = &scannertest.Probe{Callback: func(ctx context.Context, _ string) (*ffprobe.FfprobeResult, error) {
				calls++
				end()
				<-ctx.Done()
				return nil, ctx.Err()
			}}
			result := s.Start()
			if result.Status != scanner.StartStarted {
				t.Fatal(result)
			}
			scannertest.WaitForGroup(t, s.launcher.Wait, 5*time.Second, "music scan")
			assertMusicInterrupted(t, s)
			if calls != 1 {
				t.Fatalf("probe calls=%d", calls)
			}
			rows := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks")
			if rows != 0 {
				t.Fatalf("canceled scan wrote %d tracks", rows)
			}
		})
	}
}

// expiringContext is a context whose deadline passes when the test calls
// expire: Done closes and Err reports context.DeadlineExceeded.
type expiringContext struct {
	context.Context
	done   chan struct{}
	expire func()
}

func newExpiringContext() *expiringContext {
	c := &expiringContext{Context: context.Background(), done: make(chan struct{})}
	c.expire = sync.OnceFunc(func() { close(c.done) })
	return c
}

func (c *expiringContext) Done() <-chan struct{} { return c.done }

func (c *expiringContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			spotify := &cancelingMusicSpotify{cancel: cancel, phase: phase}
			s.spotify = spotify
			s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
			scan := newMusicScanContext(nil)
			scanned, _, failures := s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{{Path: musicDir + "/cancel.m4a", Ext: "m4a", Size: 1}, {Path: musicDir + "/later.m4a", Ext: "m4a", Size: 1}})
			if scanned != 0 || failures != 0 || spotify.artistCalls != 1 {
				t.Fatalf("scanned=%d errors=%d Spotify=%+v", scanned, failures, spotify)
			}
			if len(scan.spotifyArtistMisses) != 0 || len(scan.spotifyAlbumMisses) != 0 {
				t.Fatal("cancellation cached as Spotify failure")
			}
			for _, table := range []string{"tracks", "musicians", "albums", "music_spotify_matches"} {
				count := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM "+table)
				if count != 0 {
					t.Fatalf("canceled resolution wrote to %s", table)
				}
			}
		})
	}
}

func TestShutdownWaitingForPersistence(t *testing.T) {
	s := setupMusicScanner(t)
	directory := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(directory, "track.m4a"), "audio")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.scanContext = ctx
	s.currentMusicDirectory = func() sql.NullString { return sql.NullString{String: directory, Valid: true} }
	entered := make(chan struct{})
	s.ffprobe = &scannertest.Probe{Callback: func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		close(entered)
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track"}), nil
	}}
	s.tx.Mu.Lock()
	result := s.Start()
	if result.Status != scanner.StartStarted {
		s.tx.Mu.Unlock()
		t.Fatal(result)
	}
	scannertest.WaitForSignal(t, entered, 5*time.Second, "music scan reaching persistence")
	cancel()
	s.tx.Mu.Unlock()
	scannertest.WaitForGroup(t, s.launcher.Wait, 5*time.Second, "music scan")
	assertMusicInterrupted(t, s)
	rows := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks")
	if rows != 0 {
		t.Fatalf("canceled persistence wrote %d tracks", rows)
	}
}

func TestShutdownDuringTrackTransaction(t *testing.T) {
	musicDir := t.TempDir()
	// A file database survives database/sql discarding a canceled connection.
	db, queries := scannertest.OpenDB(t, filepath.Join(t.TempDir(), "music.db")+"?_foreign_keys=on")
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
	s := New(Dependencies{Now: func() time.Time { return time.Now().Add(2 * time.Minute) }, DB: db, Queries: queries, Logger: &scannertest.Logger{}, Ffprobe: &scannertest.CountingProbe{Default: testMusicMetadata()}, InvalidateCommittedTrack: func(int64) { invalidations++ }})
	scan := newMusicScanContext(nil)
	scanned, _, failures := s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{{Path: musicDir + "/interrupt.m4a", Ext: "m4a", Size: 1}})
	if scanned != 0 || failures != 0 || invalidations != 0 {
		t.Fatalf("scanned=%d failures=%d invalidations=%d", scanned, failures, invalidations)
	}
	if !reflect.DeepEqual(scan, newMusicScanContext(nil)) {
		t.Fatal("canceled transaction published caches")
	}
	for _, table := range []string{"tracks", "musicians", "albums", "musician_albums"} {
		count := scannertest.CountRows(t, db, "SELECT count(*) FROM "+table)
		if count != 0 {
			t.Fatalf("canceled transaction left %d rows in %s", count, table)
		}
	}
}
