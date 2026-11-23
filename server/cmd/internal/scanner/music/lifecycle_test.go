package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	spotifyapi "igloo/cmd/internal/spotify"

	"github.com/mattn/go-sqlite3"
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
	if ffprobeStub.audioCallsFor(mp3Path) != 2 {
		t.Fatalf("changed file audio calls = %d, want 2", ffprobeStub.audioCallsFor(mp3Path))
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
	probe := scannertest.NewGateProbe(testMusicMetadata())
	first.ffprobe = probe
	defer func() { probe.Release(); scannertest.WaitForGroup(t, first.launcher.Wait, 5*time.Second, "music scan") }()
	result := first.Start()
	if result.Status != scanner.StartStarted || result.Directory != directory {
		t.Fatalf("first start: %+v", result)
	}
	got := probe.WaitEntered(t, 5*time.Second, "scan probing the captured directory")
	if got != path || calls.Load() != 1 {
		t.Fatalf("probed %q, directory calls=%d", got, calls.Load())
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

func (s *cancelingMusicSpotify) SearchArtistByName(context.Context, string) (*spotifyapi.FullArtist, error) {
	s.artistCalls++
	if s.phase == "artist" {
		s.cancel()
		return nil, context.Canceled
	}
	return nil, nil
}

func (s *cancelingMusicSpotify) MatchAlbum(context.Context, spotifyapi.AlbumQuery) (*spotifyapi.FullAlbum, error) {
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
			for _, table := range []string{"tracks", "musicians", "albums", "music_provider_matches"} {
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
	// Released up front: the probe only reports that the scan got past it.
	probe := scannertest.NewGateProbe(testMusicMetadataWithTags(ffprobe.FormatTags{Title: "Track"}))
	probe.Release()
	s.ffprobe = probe
	s.tx.Mu.Lock()
	result := s.Start()
	if result.Status != scanner.StartStarted {
		s.tx.Mu.Unlock()
		t.Fatal(result)
	}
	probe.WaitEntered(t, 5*time.Second, "music scan reaching persistence")
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
	s := setupMusicScannerDatabase(t, filepath.Join(t.TempDir(), "music.db")+"?_foreign_keys=on")
	db := s.tx.DB
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
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.invalidateCommittedTrack = func(int64) { invalidations++ }
	scan := newMusicScanContext(nil)
	scanned, _, failures := s.processMusicFixtureBatch(t, ctx, scan, []scanner.ScanFile{{Path: musicDir + "/interrupt.m4a", Ext: "m4a", Size: 1}})
	if scanned != 0 || failures != 0 || invalidations != 0 {
		t.Fatalf("scanned=%d failures=%d invalidations=%d", scanned, failures, invalidations)
	}
	expected := newMusicScanContext(nil)
	expected.startedAt = scan.startedAt
	expected.breaker = scan.breaker
	expected.appleBreaker = scan.appleBreaker
	if !reflect.DeepEqual(scan, expected) {
		t.Fatal("canceled transaction published caches")
	}
	for _, table := range []string{"tracks", "musicians", "albums", "musician_albums"} {
		count := scannertest.CountRows(t, db, "SELECT count(*) FROM "+table)
		if count != 0 {
			t.Fatalf("canceled transaction left %d rows in %s", count, table)
		}
	}
}

// scanChanges runs one run of watcher-detected changes synchronously.
func (s *Scanner) scanChanges(directory string, changes scanner.Changes) {
	s.beginReport(changes.Scope())
	s.runMusicScan(directory, &changes)
}

func albumQueryTitles(stub *musicScannerSpotifyStub) []string {
	titles := make([]string, 0, len(stub.albumQueries))
	for _, query := range stub.albumQueries {
		titles = append(titles, query.Title)
	}
	slices.Sort(titles)
	return slices.Compact(titles)
}

func TestMusicChangesRunKeepsEnrichmentToTheTouchedAlbum(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	tags := func(title, artist, album, track string) *ffprobe.FfprobeResult {
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: title, Artist: artist, Album: album, Track: track})
	}
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A", "01.flac"), "a1")
	a2 := filepath.Join(root, "Artist A", "Album A", "02.flac")
	b1 := scannertest.WriteFile(t, filepath.Join(root, "Artist B", "Album B", "01.flac"), "b1")
	s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		a1: tags("One", "Artist A", "Album A", "1"),
		a2: tags("Two", "Artist A", "Album A", "2"),
		b1: tags("Uno", "Artist B", "Album B", "1"),
	})
	// Every lookup misses the way a 404 does, so both albums stay retry
	// candidates and every scan's retry pass would search them again.
	stub := &musicScannerSpotifyStub{artistErr: entityLookupFailure, albumErr: entityLookupFailure}
	s.spotify = stub
	s.scan(root)
	if s.Status().Imported != 2 {
		t.Fatalf("library scan: %+v", s.Status().Counts)
	}

	stub.albumQueries, stub.albumCalls, stub.artistCalls = nil, 0, 0
	scannertest.WriteFile(t, a2, "a2")
	s.scanChanges(root, scanner.Changes{Paths: []string{"Artist A/Album A/02.flac"}})
	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.Total != 1 || status.Imported != 1 {
		t.Fatalf("changes run: scope=%s %+v", status.Scope, status.Counts)
	}
	titles := albumQueryTitles(stub)
	if !slices.Equal(titles, []string{"Album A"}) {
		t.Fatalf("a run of changes searched %q; only the touched album may be searched", titles)
	}

	// A library scan still retries every pending album whose backoff has
	// elapsed; Album A has failed in two scans by now, so lift its wait first.
	_, err := s.tx.DB.Exec(`UPDATE music_provider_matches SET retry_after = NULL`)
	if err != nil {
		t.Fatal(err)
	}
	stub.albumQueries = nil
	s.scan(root)
	titles = albumQueryTitles(stub)
	if !slices.Equal(titles, []string{"Album A", "Album B"}) {
		t.Fatalf("library scan searched %q", titles)
	}
}

func TestMusicChangesRunRemovesOnlyTheReportedTracks(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Album A", "01.flac"), "a1")
	b1 := scannertest.WriteFile(t, filepath.Join(root, "Album B", "01.flac"), "b1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.scan(root)
	for _, path := range []string{a1, b1} {
		err := os.Remove(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	s.scanChanges(root, scanner.Changes{Paths: []string{"Album A"}})
	if s.Status().Deleted != 1 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks") != 1 {
		t.Fatalf("deleted=%d", s.Status().Deleted)
	}
	var kept string
	err := s.tx.DB.QueryRow("SELECT file_path FROM tracks").Scan(&kept)
	if err != nil || kept != b1 {
		t.Fatalf("kept %q, want the unreported %q: %v", kept, b1, err)
	}

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if status.Deleted != 0 || status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupSkippedEmpty {
		t.Fatalf("catch-up over an empty root: %+v", status)
	}
}

// A disk mounted inside the library that has not come up leaves its empty
// mount point; the catch-up keeps its tracks, with their likes and playlist
// entries.
func TestMusicCatchUpKeepsTheTracksOfADiskThatIsNotMounted(t *testing.T) {
	s := setupMusicScanner(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	disk := filepath.Join(root, "disk2")
	scannertest.WriteFile(t, filepath.Join(root, "disk1", "Album A", "01.flac"), "a1")
	scannertest.WriteFile(t, filepath.Join(disk, "Album B", "01.flac"), "b1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.mountPoints = func() ([]string, error) { return []string{disk}, nil }
	s.scan(root)
	err = os.RemoveAll(filepath.Join(disk, "Album B"))
	if err != nil {
		t.Fatal(err)
	}
	s.mountPoints = func() ([]string, error) { return nil, nil }

	s.scanChanges(root, scanner.Changes{CatchUp: true})
	status := s.Status()
	if status.Deleted != 0 || scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM tracks") != 2 {
		t.Fatalf("catch-up removed tracks: deleted=%d", status.Deleted)
	}
	if status.IssueCount != 1 || status.Issues[0].Reason != scanner.ReasonCleanupKeptUnmounted {
		t.Fatalf("issues = %+v", status.Issues)
	}
}

func TestMusicChangesRunHandsBackDeferredTracks(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "Album", "01.flac"), "fresh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.now = func() time.Time { return info.ModTime().Add(time.Second) }
	var deferred []string
	s.scanChanges(root, scanner.Changes{
		Paths: []string{"Album/01.flac"},
		OnDeferred: func(path string, eligible time.Time) {
			if !eligible.IsZero() {
				t.Errorf("music hands back no eligible time; got %v", eligible)
			}
			deferred = append(deferred, path)
		},
	})
	status := s.Status()
	if status.Deferred != 1 || status.Imported != 0 || !slices.Equal(deferred, []string{"Album/01.flac"}) {
		t.Fatalf("deferred=%q %+v", deferred, status.Counts)
	}
	if len(status.Issues) != 1 || status.Issues[0].Reason != scanner.ReasonFileDeferredWatched {
		t.Fatalf("issue = %+v", status.Issues)
	}
}

func TestMusicChangesRunFollowsAlbumMergesIntoTheOwner(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	tags := func(title, album, track string) *ffprobe.FfprobeResult {
		return testMusicMetadataWithTags(ffprobe.FormatTags{Title: title, Artist: "Artist A", Album: album, Track: track})
	}
	a1 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A", "01.flac"), "a1")
	a2 := scannertest.WriteFile(t, filepath.Join(root, "Artist A", "Album A (Deluxe)", "02.flac"), "a2")
	s.ffprobe = newMusicScannerFfprobeByPath(map[string]*ffprobe.FfprobeResult{
		a1: tags("One", "Album A", "1"),
		a2: tags("Two", "Album A (Deluxe)", "2"),
	})
	release := &spotifyapi.FullAlbum{
		SimpleAlbum: spotifyapi.SimpleAlbum{ID: "album", Name: "Album A", AlbumType: "album", Artists: []spotifyapi.SimpleArtist{{ID: "artist", Name: "Artist A"}}},
		Tracks: []spotifyapi.SimpleTrack{
			spotifyTrack("t1", "One", 1, 1, 180250, false),
			spotifyTrack("t2", "Two", 1, 2, 180250, false),
		},
	}
	// The deluxe title misses the way a 404 does in the library scan, so its
	// album stays a retry candidate.
	stub := &musicScannerSpotifyStub{
		artist:        &spotifyapi.FullArtist{SimpleArtist: spotifyapi.SimpleArtist{ID: "artist", Name: "Artist A"}},
		albumsByTitle: map[string]*spotifyapi.FullAlbum{"album a": release},
	}
	s.spotify = stub
	s.scan(root)
	if albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums"); albums != 2 {
		t.Fatalf("library scan albums = %d, want the two local albums", albums)
	}
	assertLinked(t, trackLinkFor(t, s, a1), "t1", false, 1)
	albumOf := func(path string) int64 {
		t.Helper()
		var id int64
		err := s.tx.DB.QueryRow("SELECT album_id FROM tracks WHERE file_path = ?", path).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	owner := albumOf(a1)

	// The run reconsiders the deluxe folder unchanged; its retry pass now
	// matches the album to the owner's release, which merges the two.
	stub.albumsByTitle["album a (deluxe)"] = release
	s.scanChanges(root, scanner.Changes{Paths: []string{"Artist A/Album A (Deluxe)"}})
	status := s.Status()
	if status.Scope != scanner.ScopeChanges || status.Unchanged != 1 || status.State != scanner.StateCompleted {
		t.Fatalf("changes run: scope=%s state=%s %+v", status.Scope, status.State, status.Counts)
	}
	if albums := scannertest.CountRows(t, s.tx.DB, "SELECT count(*) FROM albums"); albums != 1 {
		t.Fatalf("albums after the merge = %d, want one owner", albums)
	}
	if got := albumOf(a2); got != owner {
		t.Fatalf("the deluxe track moved to album %d, want the owner %d", got, owner)
	}
	// The merge moved the track to the owner; the run's track pairing must
	// follow it there rather than keep to the album it deleted.
	assertLinked(t, trackLinkFor(t, s, a2), "t2", false, 2)
}

func TestAlbumRetryCandidatesInBindsEveryTouchedAlbum(t *testing.T) {
	s := setupMusicScanner(t)
	// Four pending albums; the second and third carry Apple catalog hints, so
	// they wait for Apple only while Apple lookups are enabled. The fourth is
	// never touched, so no scoped run may return it.
	_, err := s.tx.DB.Exec(`
		INSERT INTO albums(id, title, sort_title) VALUES (1, 'One', 'One'), (2, 'Two', 'Two'), (3, 'Three', 'Three'), (4, 'Four', 'Four');
		INSERT INTO tracks(title, sort_title, file_path, file_name, container, mime_type, codec, size, track_index, duration, disc, channels, channel_layout, bit_rate, profile, album_id)
		SELECT 'Track', 'Track', '/music/' || id || '.m4a', id || '.m4a', 'm4a', 'audio/mp4', 'aac', 1, 1, 1, 1, '2', 'stereo', 1, 'LC', id FROM albums;
		INSERT INTO music_track_metadata(track_id, artist_tag, artist_key, artist_sort, album_sort, apple_album_id)
		SELECT id, '', '', '', '', 1000 + id FROM tracks WHERE album_id IN (2, 3);`)
	if err != nil {
		t.Fatal(err)
	}
	candidates := func(appleEnabled bool, albums []int64) []int64 {
		t.Helper()
		scan := newMusicScanContext(nil)
		scan.appleEnabled = appleEnabled
		scan.scope = &musicScope{albums: albums}
		rows, err := s.albumRetryCandidates(context.Background(), scan, 0)
		if err != nil {
			t.Fatalf("retry candidates of %v: %v", albums, err)
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		return ids
	}
	got := candidates(false, []int64{1, 2, 3})
	if !slices.Equal(got, []int64{1, 2, 3}) {
		t.Fatalf("with Apple off, every touched album is a Spotify candidate; got %v", got)
	}
	got = candidates(true, []int64{1, 2, 3})
	if !slices.Equal(got, []int64{1}) {
		t.Fatalf("with Apple on, hinted albums wait for Apple; got %v", got)
	}
	got = candidates(false, nil)
	if len(got) != 0 {
		t.Fatalf("a run that touched no album has no candidates; got %v", got)
	}
}

func TestMusicChangesRunThatOnlyRemovesTracksCompletes(t *testing.T) {
	s := setupMusicScanner(t)
	root := t.TempDir()
	path := scannertest.WriteFile(t, filepath.Join(root, "Album", "01.flac"), "a1")
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata()}
	s.spotify = &musicScannerSpotifyStub{artistErr: entityLookupFailure, albumErr: entityLookupFailure}
	s.scan(root)
	err := os.Remove(path)
	if err != nil {
		t.Fatal(err)
	}
	s.scanChanges(root, scanner.Changes{Paths: []string{"Album/01.flac"}})
	status := s.Status()
	if status.State != scanner.StateCompleted || status.Deleted != 1 {
		t.Fatalf("deletion-only run with Spotify configured: state=%s %+v issues=%+v", status.State, status.Counts, status.Issues)
	}
}

// A library scan's failed track stays in the status through a run of changes
// that does not look at it.
func TestMusicChangesRunKeepsTheIssuesOfFilesItDidNotLookAt(t *testing.T) {
	s := setupMusicScanner(t)
	s.ffprobe = &scannertest.CountingProbe{Default: testMusicMetadata(), Hook: func(_ context.Context, path string) (*ffprobe.FfprobeResult, error) {
		if filepath.Base(path) == "bad.flac" {
			return nil, errors.New("unreadable")
		}
		return testMusicMetadata(), nil
	}}
	root := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "Album", "bad.flac"), "bad")
	s.scan(root)
	scannertest.WriteFile(t, filepath.Join(root, "Album", "new.flac"), "new")
	s.scanChanges(root, scanner.Changes{Paths: []string{"Album/new.flac"}})
	status := s.Status()
	if status.IssueCount != 1 || status.Issues[0].Filename != "bad.flac" {
		t.Fatalf("after a run of changes elsewhere: %+v", status.Issues)
	}
}

// Each iteration scans 120 changed files against real SQLite. Rewriting the
// fixture files happens with the timer stopped, so only fingerprinting,
// database writes, resolution, and transaction cache publication are measured.
func BenchmarkMusicScan(b *testing.B) {
	for _, distinct := range []bool{false, true} {
		name := "RepeatedAlbum"
		if distinct {
			name = "DistinctTracks"
		}
		b.Run(name, func(b *testing.B) {
			musicDir := b.TempDir()
			s := setupMusicScanner(b)
			results := make(map[string]*ffprobe.FfprobeResult)
			files := make([]scanner.ScanFile, 120)
			for i := range files {
				path := filepath.Join(musicDir, fmt.Sprintf("%d.m4a", i))
				tags := ffprobe.FormatTags{Title: fmt.Sprintf("Track %d", i), Artist: "Artist", Album: "Album", Genre: "Rock"}
				if distinct {
					tags.Artist = fmt.Sprintf("Artist %d", i)
					tags.Album = fmt.Sprintf("Album %d", i)
				}
				results[path] = testMusicMetadataWithTags(tags)
				files[i] = scanner.ScanFile{Path: path, Ext: "m4a", Size: 1}
			}
			s.ffprobe = newMusicScannerFfprobeByPath(results)
			s.spotify = &musicScannerSpotifyStub{}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				for i := range files {
					files[i].Size = int64(n + 1)
				}
				prepareMusicFixtures(b, files)
				b.StartTimer()
				scanned, _, failures := s.processBatchCounts(context.Background(), newMusicScanContext(nil), files)
				if scanned != len(files) || failures != 0 {
					b.Fatalf("scanned=%d failures=%d", scanned, failures)
				}
			}
		})
	}
}
