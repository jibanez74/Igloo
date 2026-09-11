package show

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/tmdb"
)

// scan runs one complete library scan synchronously, publishing the run the
// way Start does before handing off to the scan goroutine. It reports
// cancellation and fatal failures, which the scan goroutine records on the
// run instead of returning.
func (s *Scanner) scan(directory string) error {
	s.beginReport()
	s.runShowScan(directory)
	contextErr := s.ScanContext.Err()
	if contextErr != nil {
		return contextErr
	}
	if s.Status().State == scanner.StateFailed {
		return errors.New("show scan failed")
	}
	return nil
}

// testProbe counts probes so rescans can assert that unchanged files are not
// re-probed; scannertest.Probe has no counter.
type testProbe struct {
	scannertest.NoKeyframeProbe
	calls int
	hook  func(context.Context, string) (*ffprobe.FfprobeResult, error)
}

func (p *testProbe) GetAudioMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	return p.GetMetadata(ctx, path)
}

func (p *testProbe) GetMetadata(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
	p.calls++
	if p.hook != nil {
		return p.hook(ctx, path)
	}
	var result ffprobe.FfprobeResult
	result.Format.Duration = "3600.5"
	result.Streams = []ffprobe.Stream{{Index: 3, CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080}, {Index: 5, CodecType: "audio", CodecName: "aac", Channels: 2}, {Index: 7, CodecType: "subtitle", CodecName: "subrip"}}
	return &result, nil
}
func setupScanner(t *testing.T) (*Scanner, *testProbe, string) {
	t.Helper()
	db, q := testDB(t)
	root := t.TempDir()
	probe := &testProbe{}
	s := New(Dependencies{DB: db, Queries: q, Logger: &scannertest.Logger{}, Ffprobe: probe, Now: func() time.Time { return time.Now().Add(2 * time.Minute) }, CurrentShowsDirectory: func() sql.NullString { return sql.NullString{String: root, Valid: true} }})
	return s, probe, root
}
func writeFile(t *testing.T, root, relative, data string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	err := os.MkdirAll(filepath.Dir(path), 0700)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(path, []byte(data), 0600)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func scanOK(t *testing.T, s *Scanner, root string) {
	t.Helper()
	err := s.scan(root)
	if err != nil {
		t.Fatal(err)
	}
}
func TestParsing(t *testing.T) {
	for _, tc := range []struct {
		path     string
		episodes []int
		season   int
	}{
		{"Show (2020)/Season 1/Show.s01e02.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02-E04.mp4", []int{2, 3, 4}, 1},
		{"Show/Season 1/S01E02E03.avi", []int{2, 3}, 1},
		{"Show/season 01/1x02.webm", []int{2}, 1},
		{"Breaking Bad/Season 2/Breaking Bad - S02E09 - 4 Days Out.mkv", []int{9}, 2},
		{"Show/Season 1/Show.S01E02.[1920x1080].mkv", []int{2}, 1},
		{"Show/Season 1/S01E02  -   4 Days Out.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02\t-\t4 Days Out.mkv", []int{2}, 1},
		{"Show/Season 1/1x02 - 4 Days Out.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02-E04 - 4 Days Out.mkv", []int{2, 3, 4}, 1},
		{"Show/Season 1/S01E02E03 - 1984.mkv", []int{2, 3}, 1},
		{"Show/Season 1/S01E02 - 1984.mkv", []int{2}, 1},
		{"Show/Season 1/1920x1080 S01E02.mkv", []int{2}, 1},
		{"Show/Season 1/[1920x1080] S01E02.mkv", []int{2}, 1},
		{"Show/Season 1/S01E02 1920x1080.mkv", []int{2}, 1},
		{"Show/Season 1/1920X1080 1x02 [1280x720].mkv", []int{2}, 1},
		{"Show/Season 1/[640x480][1920x1080]S01E02-E04[3840X2160].mkv", []int{2, 3, 4}, 1},
		{"Show/Season 1/[720x1280] S01E02 [480x640].mkv", []int{2}, 1},
		{"Show/Specials/S00E01.mkv", []int{1}, 0},
		{"Show/Season 0/S00E01-E03.mkv", []int{1, 2, 3}, 0},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := parseFile("/tv", filepath.Join("/tv", tc.path))
			if err != nil || !reflect.DeepEqual(got.episodes, tc.episodes) || got.season != tc.season {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	for _, path := range []string{
		"Show/Season 1/1920x1080.mkv",
		"Show/Season 1/[1920X1080].mkv",
		"Show/Season 1/[640x480] [1920x1080].mkv",
		"Show/Season 1/S01E01 1x03 [1920x1080].mkv",
		"Show/Season 1/[1920x1080] S01E01 S01E02.mkv",
		"Show/Season 1/S01E01 - 4 Days Out 1x03.mkv",
		"Show/Season 1/S01E01-03 [1920x1080].mkv",
		"Show/Season 1/S01E01-E [1920x1080].mkv",
		"Show/Season 1/S01E01- 03.mkv",
		"Show/Season 1/S01E01 -03.mkv",
		"Show/Season 1/S01E01\t-03.mkv",
		"Show/Season 1/S01E01-\t03.mkv",
		"Show/Season 1/[1920x1080] S02E01.mkv",
		"Show/Season 1/[1920x1080] S01E04-E02.mkv",
		"Show/Season 1/[1920x1080] S01E01 E03.mkv",
		"Show/Season 1/S01E01E99999.mkv",
		"Show/Season 1/S01E99999 - 4 Days Out.mkv",
		"Show/Season 1/S00001E01.mkv",
		"Show/Season 1/10001x02.mkv",
		"Show/Season 1/1x00002.mkv",
		"Show/Season 1/S01E01 19200x1080.mkv",
		"Show/Season 1/S01E01 1920x10800.mkv",
		"Show/Season 1/S01E01 a1920x1080.mkv",
		"Show/Season 1/S01E01 1920x1080p.mkv",
		"Show/Season 1/S01E01 99x1080.mkv",
		"Show/Season 1/S01E01 1920x99.mkv",
		"Show/Season 1/Season1920x1080.mkv",
		"Show/Season 1/1920x1080p.mkv",
		"Show/Season 1/[1920x1080] S01E01E99999.mkv",
		"Show/Season 1/[1920x1080] aS01E01.mkv",
		"Show/Season 1/S01E04-E02.mkv", "Show/Season 1/S02E01.mkv", "Show/Season 1/S01E01-S02E02.mkv", "Show/Season 1/S01E01 1x03.mkv", "Show/Season 1/S01E01E01.mkv", "Show/Season 1/S01E01-03.mkv", "Show/Season 1/S01E01-E.mkv", "Show/Season 1/S01E00.mkv", "Show/Season 1/S01E01E00.mkv", "Show/Season 1/S01E01 E03.mkv", "Show/Season 1/S01E01_E03.mkv", "Show/Season 1/2020-01-02.mkv", "Show/Season 1/001.mkv", "Show/Season 1/extras/S01E01.mkv", ".backup/Show/Season 1/S01E01.mkv", "Show/Season 1/.S01E01.mkv", "Show/Specials/S01E01.mkv",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := parseFile("/tv", filepath.Join("/tv", path))
			if err == nil {
				t.Fatal("accepted malformed path")
			}
		})
	}
}

func TestNumericTitlesAndResolutionsReachProbing(t *testing.T) {
	s, probe, root := setupScanner(t)
	files := []struct {
		path    string
		season  int
		episode int
	}{
		{"Breaking Bad/Season 2/Breaking Bad - S02E09 - 4 Days Out.mkv", 2, 9},
		{"Show/Season 1/Show.S01E02.[1920x1080].mkv", 1, 2},
	}
	for _, file := range files {
		writeFile(t, root, file.path, file.path)
	}
	conflict := writeFile(t, root, "Show/Season 1/S01E01 1x03 [1920x1080].mkv", "conflict")
	scanOK(t, s, root)
	if probe.calls != len(files) || countRows(t, s.DB, "show_files") != len(files) || countRows(t, s.DB, "show_episode_files") != len(files) {
		t.Fatalf("expected two probed and linked files, got %d probes", probe.calls)
	}
	for _, file := range files {
		var season, episode int
		err := s.DB.QueryRow(`SELECT ss.season_number, se.episode_number
			FROM show_files sf
			JOIN show_episode_files sef ON sef.file_id = sf.id
			JOIN show_episodes se ON se.id = sef.episode_id
			JOIN show_seasons ss ON ss.id = se.season_id
			WHERE sf.file_path = ?`, filepath.Join(root, file.path)).Scan(&season, &episode)
		if err != nil {
			t.Fatal(err)
		}
		if season != file.season || episode != file.episode {
			t.Fatalf("%s linked to season %d episode %d", file.path, season, episode)
		}
	}
	_, err := s.Queries.GetShowFileByPath(context.Background(), conflict)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("conflicting file should be excluded: %v", err)
	}
}

func TestLocalFilesCopiesFingerprintsAndCleanup(t *testing.T) {
	s, probe, root := setupScanner(t)
	combined := writeFile(t, root, "Example (2020)/Season 1/S01E01-E03.mkv", "combined")
	copyPath := writeFile(t, root, "Example (2020)/Season 1/S01E02.copy.mkv", "copy")
	writeFile(t, root, "Example (2020)/Specials/S00E01.mkv", "special")
	for _, path := range []string{".backup/Example/Season 1/S01E01.mkv", "Example (2020)/Season 1/extras/S01E04.mkv", "Example (2020)/Season 1/.S01E05.mkv", "Example (2020)/Season 1/S01E01.nfo", "Example (2020)/Season 1/S01E01.srt", "Example (2020)/Season 1/S02E01.mkv"} {
		writeFile(t, root, path, "ignore")
	}
	scanOK(t, s, root)
	if probe.calls != 3 || countRows(t, s.DB, "show_files") != 3 || countRows(t, s.DB, "show_episodes") != 4 || countRows(t, s.DB, "show_episode_files") != 5 {
		t.Fatal("wrong physical/logical counts", probe.calls)
	}
	before, err := s.Queries.GetShowFileByPath(context.Background(), combined)
	if err != nil {
		t.Fatal(err)
	}
	eps, err := s.Queries.GetShowFileEpisodes(context.Background(), before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Duration.Float64 != 3600.5 || eps[0].TmdbRuntime.Valid || eps[0].Name != "Episode 1" {
		t.Fatal("guessed episode metadata")
	}
	scanOK(t, s, root)
	if probe.calls != 3 {
		t.Fatal("unchanged scan probed")
	}
	// Same bytes and changed filesystem metadata only update the fingerprint.
	stamp := time.Now().Add(-time.Hour)
	err = os.Chtimes(combined, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	if probe.calls != 3 {
		t.Fatal("fingerprint-only update probed")
	}
	writeFile(t, root, "Example (2020)/Season 1/S01E01-E03.mkv", "different bytes")
	scanOK(t, s, root)
	after, err := s.Queries.GetShowFileByPath(context.Background(), combined)
	if err != nil {
		t.Fatal(err)
	}
	afterEps, err := s.Queries.GetShowFileEpisodes(context.Background(), after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if probe.calls != 4 || after.ID != before.ID || afterEps[1].ID != eps[1].ID {
		t.Fatal("changed file lost identity")
	}
	err = os.Remove(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	if countRows(t, s.DB, "show_episodes") != 4 || countRows(t, s.DB, "show_files") != 2 {
		t.Fatal("deleting copy removed episode")
	}
	err = os.RemoveAll(filepath.Join(root, "Example (2020)"))
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	for _, table := range []string{"shows", "show_seasons", "show_episodes", "show_files", "show_episode_files", "show_file_fingerprints", "show_tmdb_retries", "show_season_tmdb_retries", "show_episode_tmdb_retries", "show_video_streams", "show_audio_streams", "show_subtitles"} {
		if countRows(t, s.DB, table) != 0 {
			t.Fatalf("cleanup left %s", table)
		}
	}
}

func TestDeferralProbeFailureRollbackAndCancellation(t *testing.T) {
	for _, mode := range []string{"quiet", "probe", "artwork", "rollback", "unstable", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s, p, root := setupScanner(t)
			path := writeFile(t, root, "Show/Season 1/S01E01E02.mkv", "original")
			switch mode {
			case "quiet":
				s.Now = time.Now
			case "probe":
				p.hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) { return nil, errors.New("failed") }
			case "artwork":
				p.hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) {
					return &ffprobe.FfprobeResult{Streams: []ffprobe.Stream{{CodecType: "video", CodecName: "png"}}}, nil
				}
			case "rollback":
				_, err := s.DB.Exec("CREATE TRIGGER fail_tv BEFORE INSERT ON show_file_fingerprints BEGIN SELECT RAISE(ABORT,'forced'); END")
				if err != nil {
					t.Fatal(err)
				}
			case "unstable":
				p.hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) {
					err := os.WriteFile(path, []byte("changed during probe"), 0600)
					if err != nil {
						return nil, err
					}
					return &ffprobe.FfprobeResult{Streams: []ffprobe.Stream{{CodecType: "video", CodecName: "h264"}}}, nil
				}
			case "cancel":
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				s.ScanContext = ctx
				p.hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) { cancel(); return nil, ctx.Err() }
			}
			err := s.scan(root)
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("ignored cancellation", err)
			}
			for _, table := range []string{"shows", "show_seasons", "show_episodes", "show_files", "show_file_fingerprints", "show_episode_files", "show_video_streams"} {
				if countRows(t, s.DB, table) != 0 {
					t.Fatalf("partial import left %s", table)
				}
			}
		})
	}
}

func TestCleanupMissingRootChangedRootAndSymlinks(t *testing.T) {
	s, _, root := setupScanner(t)
	target := writeFile(t, t.TempDir(), "source.mkv", "target")
	link := filepath.Join(root, "Show/Season 1/S01E01.mkv")
	err := os.MkdirAll(filepath.Dir(link), 0700)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(target, link)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	err = os.Rename(root, root+"-moved")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root + "-moved") })
	err = s.scan(root)
	if err == nil || countRows(t, s.DB, "show_files") != 1 {
		t.Fatal("missing mount removed records")
	}
	err = os.Rename(root+"-moved", root)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.Queries.GetShowFileByPath(context.Background(), link)
	if err != nil {
		t.Fatal(err)
	}
	r, err := scanner.NewReconciliation(context.Background(), root, []scanner.CatalogFile{{ID: stored.ID, Path: link}})
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(root, root+"-moved")
	if err != nil {
		t.Fatal(err)
	}
	err = os.Mkdir(root, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.deleteMissing(context.Background(), r, scanner.CatalogFile{ID: stored.ID, Path: link})
	if err == nil || countRows(t, s.DB, "show_files") != 1 {
		t.Fatal("changed root removed records")
	}
	err = os.Remove(root)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Rename(root+"-moved", root)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Remove(target)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	if countRows(t, s.DB, "show_files") != 0 {
		t.Fatal("broken file symlink was not reconciled")
	}
}

func TestStartCapturesRootGuardsAndShutdown(t *testing.T) {
	s, p, root := setupScanner(t)
	writeFile(t, root, "Show/Season 1/S01E01.mkv", "file")
	entered := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ScanContext = ctx
	p.hook = func(ctx context.Context, _ string) (*ffprobe.FfprobeResult, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	result := s.Start()
	if result.Status != scanner.StartStarted {
		t.Fatal(result)
	}
	<-entered
	s.CurrentShowsDirectory = func() sql.NullString { return sql.NullString{String: t.TempDir(), Valid: true} }
	result = s.Start()
	if result.Status != scanner.StartAlreadyRunning {
		t.Fatal(result)
	}
	cancel()
	s.Wait.Wait()
	s.CurrentShowsDirectory = nil
	// A default scanner reports missing configuration without starting work.
	empty := New(Dependencies{})
	if empty.Start().Status != scanner.StartNotConfigured {
		t.Fatal("unconfigured scan started")
	}
}

func TestConcurrentScannerWrites(t *testing.T) {
	s, _, root := setupScanner(t)
	other := t.TempDir()
	writeFile(t, root, "A/Season 1/S01E01.mkv", "a")
	writeFile(t, other, "B/Season 1/S01E01.mkv", "b")
	second := New(s.Dependencies)
	second.Ffprobe = &testProbe{}
	var wg sync.WaitGroup
	for _, work := range []struct {
		s    *Scanner
		root string
	}{{s, root}, {second, other}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := work.s.scan(work.root)
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if countRows(t, s.DB, "shows") != 2 {
		t.Fatal("concurrent writes lost a show")
	}
}

type testTMDB struct {
	tmdb.TmdbInterface
	searchCalls, showCalls, seasonCalls, creditCalls int
	showHook                                         func(int) (*tmdb.TVShow, error)
	seasonHook                                       func(int, int) (*tmdb.TVSeason, error)
	creditsHook                                      func(int, int, int) (*tmdb.TVEpisodeCredits, error)
	searchHook                                       func(string, int) ([]tmdb.TVShow, error)
}

func (c *testTMDB) SearchShowsByTitleAndYear(_ context.Context, title string, years ...int) ([]tmdb.TVShow, error) {
	c.searchCalls++
	year := 0
	if len(years) > 0 {
		year = years[0]
	}
	if c.searchHook != nil {
		return c.searchHook(title, year)
	}
	return []tmdb.TVShow{{ID: 10, Name: "Example", FirstAirDate: "2020-01-01"}}, nil
}
func (c *testTMDB) GetShowDetails(_ context.Context, id int) (*tmdb.TVShow, error) {
	c.showCalls++
	if c.showHook != nil {
		return c.showHook(id)
	}
	return &tmdb.TVShow{ID: id, Name: "Enriched show", NumberOfEpisodes: 100, NumberOfSeasons: 10, AggregateCredits: tmdb.TVAggregateCredits{Cast: []tmdb.TVAggregateCast{{TVPerson: tmdb.TVPerson{ID: 1, Name: "Actor"}, Roles: []tmdb.TVRole{{Character: "A", CreditID: "a"}, {Character: "B", CreditID: "b"}}}}}}, nil
}
func (c *testTMDB) GetSeasonDetails(_ context.Context, id, season int) (*tmdb.TVSeason, error) {
	c.seasonCalls++
	if c.seasonHook != nil {
		return c.seasonHook(id, season)
	}
	return &tmdb.TVSeason{ID: 100 + season, SeasonNumber: season, Name: "Enriched season", Episodes: []tmdb.TVEpisode{{ID: 1001, SeasonNumber: season, EpisodeNumber: 1, Name: "First", Runtime: 40}, {ID: 1002, SeasonNumber: season, EpisodeNumber: 2, Name: "Second", Runtime: 41}, {ID: 1003, SeasonNumber: season, EpisodeNumber: 3, Name: "Remote only"}}}, nil
}
func (c *testTMDB) GetEpisodeCredits(_ context.Context, id, season, episode int) (*tmdb.TVEpisodeCredits, error) {
	c.creditCalls++
	if c.creditsHook != nil {
		return c.creditsHook(id, season, episode)
	}
	return &tmdb.TVEpisodeCredits{ID: 1000 + episode, GuestStars: []tmdb.TVCastCredit{{TVPerson: tmdb.TVPerson{ID: 2, Name: "Guest"}, Character: "Guest"}}}, nil
}

func TestOfflineImportThenGroupedEnrichmentAndPartialFailure(t *testing.T) {
	s, p, root := setupScanner(t)
	writeFile(t, root, "Example (2020)/Season 1/S01E01E02.mkv", "file")
	scanOK(t, s, root)
	if countRows(t, s.DB, "show_episode_tmdb_retries") != 2 {
		t.Fatal("offline retry markers missing")
	}
	client := &testTMDB{}
	client.creditsHook = func(_, _, ep int) (*tmdb.TVEpisodeCredits, error) {
		if ep == 2 {
			return nil, errors.New("offline")
		}
		return &tmdb.TVEpisodeCredits{ID: 1001}, nil
	}
	s.Tmdb = client
	scanOK(t, s, root)
	if p.calls != 1 || client.showCalls != 1 || client.seasonCalls != 1 || client.creditCalls != 2 || countRows(t, s.DB, "show_episode_tmdb_retries") != 1 || countRows(t, s.DB, "show_episodes") != 2 {
		t.Fatal("partial enrichment counts", client)
	}
	if countRows(t, s.DB, "show_cast") != 2 {
		t.Fatal("aggregate roles collapsed")
	}
	client.creditsHook = nil
	scanOK(t, s, root)
	if client.showCalls != 1 || client.seasonCalls != 2 || client.creditCalls != 3 || p.calls != 1 {
		t.Fatal("retry repeated successful entity or probe", client, p.calls)
	}
	pending, err := s.Queries.CountShowRetries(context.Background())
	if err != nil || pending != 0 {
		t.Fatal("retry not cleared", pending, err)
	}
	scanOK(t, s, root)
	if client.showCalls != 1 || client.seasonCalls != 2 || client.creditCalls != 3 {
		t.Fatal("successful unchanged metadata refreshed")
	}
	// A changed file queues all represented owners, and a failed stored-ID
	// lookup cannot rematch or replace successful descriptive metadata.
	writeFile(t, root, "Example (2020)/Season 1/S01E01E02.mkv", "changed file")
	client.showHook = func(id int) (*tmdb.TVShow, error) {
		if id != 10 {
			t.Error("lost identity")
		}
		return nil, errors.New("404")
	}
	scanOK(t, s, root)
	stored, err := s.Queries.GetShow(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Enriched show" || client.searchCalls != 1 || client.seasonCalls != 2 || p.calls != 2 {
		t.Fatal("failed ID lookup changed metadata or retried descendants")
	}
}

func TestMissingEpisodeAndStaleResponse(t *testing.T) {
	for _, mode := range []string{"missing", "identity", "rollback", "deleted", "credits_id"} {
		t.Run(mode, func(t *testing.T) {
			s, _, root := setupScanner(t)
			writeFile(t, root, "Example/Season 1/S01E01E02.mkv", "file")
			scanOK(t, s, root)
			client := &testTMDB{}
			s.Tmdb = client
			switch mode {
			case "missing":
				client.seasonHook = func(_, season int) (*tmdb.TVSeason, error) { return &tmdb.TVSeason{ID: 101, SeasonNumber: season}, nil }
			case "identity":
				client.showHook = func(id int) (*tmdb.TVShow, error) {
					_, err := s.DB.Exec("UPDATE shows SET tmdb_id=99,name='new identity'")
					return &tmdb.TVShow{ID: id, Name: "stale"}, err
				}
			case "deleted":
				client.showHook = func(id int) (*tmdb.TVShow, error) {
					_, err := s.DB.Exec("DELETE FROM shows")
					return &tmdb.TVShow{ID: id, Name: "stale"}, err
				}
			case "rollback":
				_, err := s.DB.Exec("CREATE TRIGGER fail_cast BEFORE INSERT ON show_cast BEGIN SELECT RAISE(ABORT,'forced'); END")
				if err != nil {
					t.Fatal(err)
				}
			case "credits_id":
				client.creditsHook = func(int, int, int) (*tmdb.TVEpisodeCredits, error) { return &tmdb.TVEpisodeCredits{ID: 777}, nil }
			}
			scanOK(t, s, root)
			if mode == "deleted" {
				if countRows(t, s.DB, "shows") != 0 {
					t.Fatal("stale response recreated catalog")
				}
				return
			}
			if countRows(t, s.DB, "show_episode_tmdb_retries") != 2 {
				t.Fatal("failure cleared episode retries")
			}
			if mode == "rollback" && (countRows(t, s.DB, "artist") != 0 || countRows(t, s.DB, "show_tmdb_retries") != 1) {
				t.Fatal("metadata transaction leaked")
			}
			if mode == "identity" {
				got, err := s.Queries.GetShow(context.Background(), 1)
				if err != nil || got.Name != "new identity" || got.TmdbID.Int64 != 99 {
					t.Fatal("stale response replaced identity", err)
				}
			}
		})
	}
}

func TestShowRankingAmbiguousYearAndDeterministicTies(t *testing.T) {
	s, _, _ := setupScanner(t)
	client := &testTMDB{}
	client.searchHook = func(title string, year int) ([]tmdb.TVShow, error) {
		return []tmdb.TVShow{{ID: 30, Name: "Space Adventures", FirstAirDate: "1999-01-01"}, {ID: 20, Name: "Space 1999", FirstAirDate: "1975-01-01"}, {ID: 10, Name: "Space 1999", FirstAirDate: "1975-01-01"}}, nil
	}
	s.Tmdb = client
	result, err := s.lookupShow(context.Background(), database.Show{DirectoryPath: "/tv/Space 1999"})
	if err != nil || result.ID != 10 || client.searchCalls != 2 {
		t.Fatal("ambiguous ranking", result, err, client.searchCalls)
	}
}

func TestTechnicalFieldsAndFileOwnedChapters(t *testing.T) {
	s, p, root := setupScanner(t)
	writeFile(t, root, "Show/Season 1/S01E01E02.mkv", "file")
	p.hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) {
		return &ffprobe.FfprobeResult{
			Streams: []ffprobe.Stream{
				{Index: 0, CodecType: "video", CodecName: "mjpeg", Disposition: ffprobe.StreamDisposition{AttachedPic: 1}},
				{Index: 2, CodecType: "video", CodecName: "hevc", Profile: "Main 10", Level: 153, BitDepth: "10", Width: 1920, Height: 1080, CodedWidth: 1920, CodedHeight: 1088, FrameRate: "24000/1001", AvgFrameRate: "24000/1001", PixelFormat: "yuv420p10le", ColorTransfer: "smpte2084", FieldOrder: "progressive", SideDataList: []ffprobe.StreamSideData{{SideDataType: "Display Matrix", Rotation: 0}}},
				{Index: 4, CodecType: "audio", CodecName: "aac", SampleRate: "48000", Channels: 6, ChannelLayout: "5.1", Tags: ffprobe.StreamTags{Language: "eng", Title: "Main"}, Disposition: ffprobe.StreamDisposition{Default: 1}},
				{Index: 6, CodecType: "subtitle", CodecName: "subrip", Disposition: ffprobe.StreamDisposition{Forced: 1, Default: 1}},
			}, Chapters: []ffprobe.Chapter{{StartTime: "573.114208"}, {StartTime: "12.000000"}},
		}, nil
	}
	scanOK(t, s, root)
	var index, level, depth, codedHeight int
	var rotation sql.NullInt64
	var frameRate float64
	var transfer string
	err := s.DB.QueryRow("SELECT stream_index,codec_level,bit_depth,coded_height,rotation,frame_rate,color_transfer FROM show_video_streams").Scan(&index, &level, &depth, &codedHeight, &rotation, &frameRate, &transfer)
	if err != nil {
		t.Fatal(err)
	}
	if index != 2 || level != 153 || depth != 10 || codedHeight != 1088 || !rotation.Valid || rotation.Int64 != 0 || frameRate < 23.97 || frameRate > 23.98 || transfer != "smpte2084" {
		t.Fatal("lost video technical fields")
	}
	var sampleRate, channels int
	var isDefault, isForced bool
	err = s.DB.QueryRow("SELECT stream_index,sample_rate,channels,is_default FROM show_audio_streams").Scan(&index, &sampleRate, &channels, &isDefault)
	if err != nil || index != 4 || sampleRate != 48000 || channels != 6 || !isDefault {
		t.Fatal("lost audio technical fields", err)
	}
	err = s.DB.QueryRow("SELECT stream_index,is_default,is_forced FROM show_subtitles").Scan(&index, &isDefault, &isForced)
	if err != nil || index != 6 || !isDefault || !isForced {
		t.Fatal("lost subtitle technical fields", err)
	}
	var first, last int
	err = s.DB.QueryRow("SELECT min(start_time),max(start_time) FROM show_chapters").Scan(&first, &last)
	if err != nil || first != 12 || last != 573 || countRows(t, s.DB, "show_chapters") != 2 {
		t.Fatal("chapters duplicated across episodes or incorrect units", err)
	}
}

func TestCleanupPermissionFailureAndDeletionRollback(t *testing.T) {
	for _, mode := range []string{"permission", "rollback"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "permission" && os.Geteuid() == 0 {
				t.Skip("root bypasses directory permissions")
			}
			s, _, root := setupScanner(t)
			path := writeFile(t, root, "Show/Season 1/S01E01.mkv", "file")
			scanOK(t, s, root)
			err := os.Remove(path)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "permission" {
				err = os.Chmod(filepath.Dir(path), 0000)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(filepath.Dir(path), 0700) })
			} else {
				_, err = s.DB.Exec("CREATE TRIGGER fail_delete BEFORE DELETE ON show_episodes BEGIN SELECT RAISE(ABORT,'forced'); END")
				if err != nil {
					t.Fatal(err)
				}
			}
			err = s.scan(root)
			if mode == "rollback" && err == nil {
				t.Fatal("deletion unexpectedly succeeded")
			}
			if countRows(t, s.DB, "show_files") != 1 || countRows(t, s.DB, "show_episode_files") != 1 {
				t.Fatal("unsafe cleanup removed records")
			}
		})
	}
}
