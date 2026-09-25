package show

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
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

func defaultProbeResult() *ffprobe.FfprobeResult {
	var result ffprobe.FfprobeResult
	result.Format.Duration = "3600.5"
	result.Streams = []ffprobe.Stream{{Index: 3, CodecType: "video", CodecName: "h264", Width: 1920, Height: 1080}, {Index: 5, CodecType: "audio", CodecName: "aac", Channels: 2}, {Index: 7, CodecType: "subtitle", CodecName: "subrip"}}
	return &result
}

func setupScanner(t *testing.T) (*Scanner, *scannertest.CountingProbe, string) {
	t.Helper()
	db, q := scannertest.OpenDB(t, ":memory:?_foreign_keys=on")
	root := t.TempDir()
	probe := &scannertest.CountingProbe{Default: defaultProbeResult()}
	s := New(Dependencies{DB: db, Queries: q, Logger: &scannertest.Logger{}, Ffprobe: probe, Now: func() time.Time { return time.Now().Add(2 * time.Minute) }, CurrentShowsDirectory: func() sql.NullString { return sql.NullString{String: root, Valid: true} }})
	return s, probe, root
}
func scanOK(t *testing.T, s *Scanner, root string) {
	t.Helper()
	err := s.scan(root)
	if err != nil {
		t.Fatal(err)
	}
}
func TestLocalFilesCopiesFingerprintsAndCleanup(t *testing.T) {
	s, probe, root := setupScanner(t)
	combined := scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01-E03.mkv"), "combined")
	copyPath := scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E02.copy.mkv"), "copy")
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Specials/S00E01.mkv"), "special")
	for _, path := range []string{".backup/Example/Season 1/S01E01.mkv", "Example (2020)/Season 1/extras/S01E04.mkv", "Example (2020)/Season 1/.S01E05.mkv", "Example (2020)/Season 1/S01E01.nfo", "Example (2020)/Season 1/S01E01.srt", "Example (2020)/Season 1/S02E01.mkv"} {
		scannertest.WriteFile(t, filepath.Join(root, path), "ignore")
	}
	scanOK(t, s, root)
	if probe.Calls() != 3 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 3 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episodes") != 4 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_files") != 5 {
		t.Fatal("wrong physical/logical counts", probe.Calls())
	}
	before, err := s.Queries.GetShowFileByPath(context.Background(), combined)
	if err != nil {
		t.Fatal(err)
	}
	eps := fileEpisodes(t, s.DB, before.ID)
	if before.Duration.Float64 != 3600.5 || eps[0].tmdbRuntime.Valid || eps[0].name != "Episode 1" {
		t.Fatal("guessed episode metadata")
	}
	scanOK(t, s, root)
	if probe.Calls() != 3 {
		t.Fatal("unchanged scan probed")
	}
	// Changed filesystem metadata re-probes without reading content (no
	// hashing, like movies) and keeps the file and episode identities.
	stamp := time.Now().Add(-time.Hour)
	err = os.Chtimes(combined, stamp, stamp)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	if probe.Calls() != 4 || s.Status().Updated != 1 {
		t.Fatal("touched file was not re-probed once", probe.Calls(), s.Status().Updated)
	}
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01-E03.mkv"), "different bytes")
	scanOK(t, s, root)
	after, err := s.Queries.GetShowFileByPath(context.Background(), combined)
	if err != nil {
		t.Fatal(err)
	}
	afterEps := fileEpisodes(t, s.DB, after.ID)
	if probe.Calls() != 5 || after.ID != before.ID || afterEps[1].id != eps[1].id {
		t.Fatal("changed file lost identity")
	}
	err = os.Remove(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episodes") != 4 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 2 {
		t.Fatal("deleting copy removed episode")
	}
	err = os.RemoveAll(filepath.Join(root, "Example (2020)"))
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	for _, table := range []string{"shows", "show_seasons", "show_episodes", "show_files", "show_episode_files", "show_file_fingerprints", "show_tmdb_retries", "show_season_tmdb_retries", "show_episode_tmdb_retries", "show_video_streams", "show_audio_streams", "show_subtitles"} {
		if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM "+table) != 0 {
			t.Fatalf("cleanup left %s", table)
		}
	}
}

func TestDeferralProbeFailureRollbackAndCancellation(t *testing.T) {
	for _, mode := range []string{"quiet", "probe", "artwork", "rollback", "unstable", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s, p, root := setupScanner(t)
			path := scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01E02.mkv"), "original")
			switch mode {
			case "quiet":
				s.Now = time.Now
			case "probe":
				p.Hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) { return nil, errors.New("failed") }
			case "artwork":
				p.Hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) {
					return &ffprobe.FfprobeResult{Streams: []ffprobe.Stream{{CodecType: "video", CodecName: "png"}}}, nil
				}
			case "rollback":
				_, err := s.DB.Exec("CREATE TRIGGER fail_tv BEFORE INSERT ON show_file_fingerprints BEGIN SELECT RAISE(ABORT,'forced'); END")
				if err != nil {
					t.Fatal(err)
				}
			case "unstable":
				p.Hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) {
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
				p.Hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) { cancel(); return nil, ctx.Err() }
			}
			err := s.scan(root)
			if mode == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal("ignored cancellation", err)
			}
			for _, table := range []string{"shows", "show_seasons", "show_episodes", "show_files", "show_file_fingerprints", "show_episode_files", "show_video_streams"} {
				if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM "+table) != 0 {
					t.Fatalf("partial import left %s", table)
				}
			}
		})
	}
}

func TestCleanupMissingRootChangedRootAndSymlinks(t *testing.T) {
	s, _, root := setupScanner(t)
	target := scannertest.WriteFile(t, filepath.Join(t.TempDir(), "source.mkv"), "target")
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
	if err == nil || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 {
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
	if err == nil || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 {
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
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 0 {
		t.Fatal("broken file symlink was not reconciled")
	}
}

func TestStartCapturesRootGuardsAndShutdown(t *testing.T) {
	s, p, root := setupScanner(t)
	path := scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "file")
	// Only the first read returns root, so a scan that re-read the setting
	// after Start would walk a directory holding none of the fixture.
	calls := atomic.Int32{}
	s.CurrentShowsDirectory = func() sql.NullString {
		if calls.Add(1) == 1 {
			return sql.NullString{String: root, Valid: true}
		}
		return sql.NullString{String: "/changed/shows", Valid: true}
	}
	entered := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ScanContext = ctx
	p.Hook = func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
		entered <- path
		<-ctx.Done()
		return nil, ctx.Err()
	}
	result := s.Start()
	if result.Status != scanner.StartStarted || result.Directory != root {
		t.Fatalf("first start: %+v", result)
	}
	select {
	case got := <-entered:
		if got != path || calls.Load() != 1 {
			t.Fatalf("probed %q, directory calls=%d; want %q read once", got, calls.Load(), path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("show scan did not probe the captured directory")
	}
	result = s.Start()
	if result.Status != scanner.StartAlreadyRunning {
		t.Fatal(result)
	}
	cancel()
	scannertest.WaitForGroup(t, s.Wait, 5*time.Second, "show scan stop")
	// A default scanner reports missing configuration without starting work.
	empty := New(Dependencies{})
	if empty.Start().Status != scanner.StartNotConfigured {
		t.Fatal("unconfigured scan started")
	}
}

func TestConcurrentScannerWrites(t *testing.T) {
	s, _, root := setupScanner(t)
	other := t.TempDir()
	scannertest.WriteFile(t, filepath.Join(root, "A/Season 1/S01E01.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(other, "B/Season 1/S01E01.mkv"), "b")
	second := New(s.Dependencies)
	second.Ffprobe = &scannertest.CountingProbe{Default: defaultProbeResult()}
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
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows") != 2 {
		t.Fatal("concurrent writes lost a show")
	}
}

type testTMDB struct {
	tmdb.TmdbInterface
	searchCalls, showCalls, seasonCalls int
	searchCtx                           context.Context
	showHook                            func(int) (*tmdb.TVShow, error)
	seasonHook                          func(int, int) (*tmdb.TVSeason, error)
	searchHook                          func(string, int) ([]tmdb.TVShow, error)
}

func (c *testTMDB) SearchShowsByTitleAndYear(ctx context.Context, title string, years ...int) ([]tmdb.TVShow, error) {
	c.searchCalls++
	c.searchCtx = ctx
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
	guest := []tmdb.TVCastCredit{{TVPerson: tmdb.TVPerson{ID: 2, Name: "Guest"}, Character: "Guest"}}
	return &tmdb.TVSeason{ID: 100 + season, SeasonNumber: season, Name: "Enriched season", Episodes: []tmdb.TVEpisode{{ID: 1001, SeasonNumber: season, EpisodeNumber: 1, Name: "First", Runtime: 40, GuestStars: guest}, {ID: 1002, SeasonNumber: season, EpisodeNumber: 2, Name: "Second", Runtime: 41, GuestStars: guest}, {ID: 1003, SeasonNumber: season, EpisodeNumber: 3, Name: "Remote only"}}}, nil
}
func TestOfflineImportThenGroupedEnrichmentAndPartialFailure(t *testing.T) {
	s, p, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "file")
	scanOK(t, s, root)
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_tmdb_retries") != 2 {
		t.Fatal("offline retry markers missing")
	}
	client := &testTMDB{}
	// A season payload that omits episode 2 leaves that episode pending.
	client.seasonHook = func(_, season int) (*tmdb.TVSeason, error) {
		full, _ := (&testTMDB{}).GetSeasonDetails(context.Background(), 0, season)
		full.Episodes = full.Episodes[:1]
		return full, nil
	}
	s.Tmdb = client
	scanOK(t, s, root)
	if p.Calls() != 1 || client.showCalls != 1 || client.seasonCalls != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_tmdb_retries") != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episodes") != 2 {
		t.Fatal("partial enrichment counts", client)
	}
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_cast") != 2 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_guest_cast") != 1 {
		t.Fatal("aggregate roles collapsed or episode guest cast missing")
	}
	client.seasonHook = nil
	scanOK(t, s, root)
	if client.showCalls != 1 || client.seasonCalls != 2 || p.Calls() != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_guest_cast") != 2 {
		t.Fatal("retry repeated successful entity or probe", client, p.Calls())
	}
	pending, err := s.Queries.CountShowRetries(context.Background())
	if err != nil || pending != 0 {
		t.Fatal("retry not cleared", pending, err)
	}
	scanOK(t, s, root)
	if client.showCalls != 1 || client.seasonCalls != 2 {
		t.Fatal("successful unchanged metadata refreshed")
	}
	// A changed file re-probes but never re-queues matched entities: no TMDB
	// call is made and the descriptions stay.
	scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01E02.mkv"), "changed file")
	scanOK(t, s, root)
	pending, err = s.Queries.CountShowRetries(context.Background())
	if err != nil || pending != 0 || client.showCalls != 1 || client.seasonCalls != 2 || p.Calls() != 2 || s.Status().EnrichmentTotal != 0 {
		t.Fatal("technical change re-queued matched metadata", pending, client, p.Calls())
	}
	// A retry marker on a matched show fetches its stored ID directly; a failed
	// lookup cannot rematch or replace successful descriptive metadata, and it
	// defers the show's descendants rather than failing them.
	_, err = s.DB.Exec("INSERT INTO show_tmdb_retries (show_id) VALUES (1)")
	if err != nil {
		t.Fatal(err)
	}
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
	status := s.Status()
	if stored.Name != "Enriched show" || client.searchCalls != 1 || client.seasonCalls != 2 || status.EnrichmentFailed != 1 || status.EnrichmentTotal != 1 || status.PendingEnrichment != 1 {
		t.Fatalf("failed ID lookup changed metadata or retried descendants: %+v", status)
	}
}

func TestMissingEpisodeAndStaleResponse(t *testing.T) {
	for _, mode := range []string{"missing", "identity", "rollback", "deleted"} {
		t.Run(mode, func(t *testing.T) {
			s, _, root := setupScanner(t)
			scannertest.WriteFile(t, filepath.Join(root, "Example/Season 1/S01E01E02.mkv"), "file")
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
			}
			scanOK(t, s, root)
			if mode == "deleted" {
				if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows") != 0 {
					t.Fatal("stale response recreated catalog")
				}
				return
			}
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_tmdb_retries") != 2 {
				t.Fatal("failure cleared episode retries")
			}
			// Episodes TMDB does not list yet stay pending without an outcome.
			if mode == "missing" {
				status := s.Status()
				if status.EnrichmentFailed != 0 || status.EnrichmentUnmatched != 0 || status.EnrichmentProcessed != status.EnrichmentTotal || status.EnrichmentTotal != 2 {
					t.Fatalf("missing episodes counted as an outcome: %+v", status)
				}
			}
			// The trigger aborts only the cast insert, and SQLite rolls a RAISE
			// back statement-wide, so the rolled-back transaction is visible in
			// the writes that preceded it: the artist upserted for that cast row
			// and the show name the update already replaced.
			if mode == "rollback" {
				got, err := s.Queries.GetShow(context.Background(), 1)
				if err != nil || got.Name != "Example" || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM artist") != 0 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_tmdb_retries") != 1 {
					t.Fatal("metadata transaction leaked", err)
				}
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
	result, err := s.lookupShow(context.Background(), &providerBreaker{}, database.Show{DirectoryPath: "/tv/Space 1999"})
	if err != nil || result.ID != 10 || client.searchCalls != 2 {
		t.Fatal("ambiguous ranking", result, err, client.searchCalls)
	}
	deadline, ok := client.searchCtx.Deadline()
	if !ok || time.Until(deadline) > scanner.TmdbLookupTimeout {
		t.Fatal("lookup ran without the shared timeout budget")
	}
}

func TestTechnicalFieldsAndFileOwnedChapters(t *testing.T) {
	s, p, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01E02.mkv"), "file")
	p.Hook = func(context.Context, string) (*ffprobe.FfprobeResult, error) {
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
	if err != nil || first != 12 || last != 573 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_chapters") != 2 {
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
			path := scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "file")
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
			if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episode_files") != 1 {
				t.Fatal("unsafe cleanup removed records")
			}
		})
	}
}

// Episode files may sit directly in the show folder, and season folders may
// be "S2" or carry a title; every layout feeds one show. The status counters
// served by the API follow files for the local phase and episodes separately.
func TestLayoutsShareOneShowAndReportCounters(t *testing.T) {
	s, probe, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Show.S01E01.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 1/S01E02.mkv"), "b")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/S2/S02E01.mkv"), "c")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Season 3 - The End/S03E01E02.mkv"), "d")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/Show.S00E01.mkv"), "special")
	scannertest.WriteFile(t, filepath.Join(root, "Show (2020)/S01/S02E01.mkv"), "mismatch")
	scanOK(t, s, root)
	status := s.Status()
	if probe.Calls() != 5 || status.Total != 6 || status.Processed != 6 || status.Imported != 5 || status.Failed != 1 || status.Episodes != 6 || status.State != scanner.StateCompletedWithIssues {
		t.Fatalf("status %+v probes=%d", status, probe.Calls())
	}
	if scannertest.CountRows(t, s.DB, "SELECT count(*) FROM shows") != 1 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_seasons") != 4 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_episodes") != 6 {
		t.Fatal("layouts did not share one show")
	}
	scanOK(t, s, root)
	status = s.Status()
	if probe.Calls() != 5 || status.Unchanged != 5 || status.Episodes != 6 || status.Updated != 0 {
		t.Fatalf("rescan %+v probes=%d", status, probe.Calls())
	}
}

// Two workers probe at once while the coordinator alone persists; the probe
// hook only returns once both files are in flight.
func TestWorkersProbeConcurrently(t *testing.T) {
	s, probe, root := setupScanner(t)
	scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01.mkv"), "a")
	scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E02.mkv"), "b")
	var mu sync.Mutex
	inFlight := 0
	release := make(chan struct{})
	probe.Hook = func(ctx context.Context, path string) (*ffprobe.FfprobeResult, error) {
		mu.Lock()
		inFlight++
		both := inFlight == 2
		mu.Unlock()
		if both {
			close(release)
		}
		select {
		case <-release:
			return defaultProbeResult(), nil
		case <-time.After(5 * time.Second):
			return nil, errors.New("second probe never started")
		}
	}
	scanOK(t, s, root)
	if probe.Calls() != 2 || scannertest.CountRows(t, s.DB, "SELECT count(*) FROM show_files") != 2 || s.Status().Imported != 2 {
		t.Fatalf("%+v", s.Status())
	}
}

// Workers must not touch the database: the pool is pinned to one connection,
// so a worker query would wait behind the coordinator's open transaction.
func TestPrepareFileStaysOffTheDatabase(t *testing.T) {
	s, _, root := setupScanner(t)
	path := scannertest.WriteFile(t, filepath.Join(root, "Show/Season 1/S01E01E02.mkv"), "a")
	s.DB.Close()
	result := s.prepareFile(context.Background(), root, probeJob{file: scanner.ScanFile{Path: path, Ext: "mkv"}})
	if result.err != nil || result.info == nil || !reflect.DeepEqual(result.local.episodes, []int{1, 2}) {
		t.Fatalf("prepare used the database: err=%v local=%+v", result.err, result.local)
	}
	result.inspection.Close()
}

// The API drops its per-file runtime caches from the after-commit hook, so
// the hook must fire with the file and every episode it backs, on a rescan
// and on deletion, and never before the rows are durable.
func TestInvalidateCommittedShowFileHookFiresAfterCommit(t *testing.T) {
	s, _, root := setupScanner(t)
	type call struct {
		fileID   int64
		episodes []int64
	}
	var calls []call
	s.InvalidateCommittedShowFile = func(fileID int64, episodeIDs []int64) {
		calls = append(calls, call{fileID: fileID, episodes: append([]int64(nil), episodeIDs...)})
	}
	combined := scannertest.WriteFile(t, filepath.Join(root, "Example (2020)/Season 1/S01E01-E02.mkv"), "combined")
	scanOK(t, s, root)
	file, err := s.Queries.GetShowFileByPath(context.Background(), combined)
	if err != nil {
		t.Fatal(err)
	}
	eps := fileEpisodes(t, s.DB, file.ID)
	if len(calls) != 1 || calls[0].fileID != file.ID || len(calls[0].episodes) != 2 || calls[0].episodes[0] != eps[0].id || calls[0].episodes[1] != eps[1].id {
		t.Fatalf("import hook calls = %+v, want one call for file %d with episodes %v", calls, file.ID, eps)
	}
	scanOK(t, s, root)
	if len(calls) != 1 {
		t.Fatal("an unchanged scan invalidated caches")
	}
	err = os.Remove(combined)
	if err != nil {
		t.Fatal(err)
	}
	scanOK(t, s, root)
	if len(calls) != 2 || calls[1].fileID != file.ID || len(calls[1].episodes) != 2 {
		t.Fatalf("deletion hook calls = %+v, want a second call for file %d with both episodes", calls, file.ID)
	}
}
