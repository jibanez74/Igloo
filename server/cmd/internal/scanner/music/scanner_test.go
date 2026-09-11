package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/sqlc"

	_ "github.com/mattn/go-sqlite3"
	spotifylib "github.com/zmb3/spotify/v2"
)

func setupMusicScanner(t testing.TB) *Scanner {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	_, err = db.Exec(sqlc.Schema)
	if err != nil {
		db.Close()
		t.Fatalf("initialize schema: %v", err)
	}
	queries, err := database.Prepare(context.Background(), db)
	if err != nil {
		db.Close()
		t.Fatalf("prepare queries: %v", err)
	}
	t.Cleanup(func() { queries.Close() })
	return New(Dependencies{DB: db, Queries: queries, Logger: &scannertest.Logger{}, Now: func() time.Time { return time.Now().Add(2 * time.Minute) }})
}

func (app *Scanner) processMusicBatchForTest(t testing.TB, ctx context.Context, files []scanner.ScanFile) (scanned, skipped, errCount int) {
	scanIndex, _, err := app.loadMusicScanIndex(ctx)
	if err != nil {
		app.logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return 0, 0, len(files)
	}

	return app.processMusicFixtureBatch(t, ctx, newMusicScanContext(scanIndex), files)
}

func testMusicMetadata() *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "180",
			BitRate:  "256000",
			Tags: ffprobe.FormatTags{
				Title:  "Test Track",
				Artist: "Test Artist",
				Album:  "Test Album",
				Track:  "1/10",
			},
		},
		Streams: []ffprobe.Stream{
			{
				Index:         0,
				CodecName:     "aac",
				CodecType:     "audio",
				Channels:      2,
				ChannelLayout: "stereo",
			},
		},
	}
}

func testMusicMetadataWithTags(tags ffprobe.FormatTags) *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: "180.250",
			BitRate:  "256000",
			Tags:     tags,
		},
		Streams: []ffprobe.Stream{
			{
				Index:         0,
				CodecName:     "aac",
				CodecType:     "audio",
				Profile:       "LC",
				Channels:      2,
				ChannelLayout: "stereo",
				Tags: ffprobe.StreamTags{
					Language: "eng",
				},
			},
		},
	}
}

type countingMusicScannerFfprobe struct {
	scannertest.NoKeyframeProbe
	result *ffprobe.FfprobeResult
	calls  int
}

func (s *countingMusicScannerFfprobe) GetMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	return s.result, nil
}

func (s *countingMusicScannerFfprobe) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	return s.result, nil
}

type cancelingMusicScannerFfprobe struct {
	scannertest.NoKeyframeProbe
	cancel context.CancelFunc
	calls  int
}

func (s *cancelingMusicScannerFfprobe) GetMetadata(_ context.Context, _ string) (*ffprobe.FfprobeResult, error) {
	return nil, errors.New("generic metadata probing is not expected")
}

func (s *cancelingMusicScannerFfprobe) GetAudioMetadata(_ context.Context, _ string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	s.cancel()
	return nil, context.Canceled
}

type failingPathMusicScannerFfprobe struct {
	scannertest.NoKeyframeProbe
	result      *ffprobe.FfprobeResult
	failingPath string
	calls       int
}

func (s *failingPathMusicScannerFfprobe) GetMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	if filePath == s.failingPath {
		return nil, errors.New("ffprobe failed")
	}

	return s.result, nil
}

func (s *failingPathMusicScannerFfprobe) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.calls++
	if filePath == s.failingPath {
		return nil, errors.New("ffprobe failed")
	}

	return s.result, nil
}

type musicScannerFfprobeByPath struct {
	scannertest.NoKeyframeProbe
	results       map[string]*ffprobe.FfprobeResult
	metadataCalls map[string]int
	audioCalls    map[string]int
}

func newMusicScannerFfprobeByPath(results map[string]*ffprobe.FfprobeResult) *musicScannerFfprobeByPath {
	return &musicScannerFfprobeByPath{
		results:       results,
		metadataCalls: make(map[string]int),
		audioCalls:    make(map[string]int),
	}
}

func (s *musicScannerFfprobeByPath) GetMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.metadataCalls[filePath]++
	return s.resultForPath(filePath)
}

func (s *musicScannerFfprobeByPath) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.audioCalls[filePath]++
	return s.resultForPath(filePath)
}

func (s *musicScannerFfprobeByPath) resultForPath(filePath string) (*ffprobe.FfprobeResult, error) {
	result, ok := s.results[filePath]
	if !ok {
		return nil, fmt.Errorf("unexpected ffprobe path: %s", filePath)
	}

	return result, nil
}

func (s *musicScannerFfprobeByPath) totalAudioCalls() int {
	total := 0
	for _, calls := range s.audioCalls {
		total += calls
	}
	return total
}

func (s *musicScannerFfprobeByPath) totalMetadataCalls() int {
	total := 0
	for _, calls := range s.metadataCalls {
		total += calls
	}
	return total
}

type musicScannerSpotifyStub struct {
	artist *spotifylib.FullArtist
	// artistsByName, when set, answers per name (lowercased); a name it does
	// not carry fails the lookup, so one stub can model a compound credit where
	// only some members match.
	artistsByName map[string]*spotifylib.FullArtist
	artistErr     error
	artistCalls   int
	album         *spotifylib.FullAlbum
	albumErr      error
	albumCalls    int
	clearCalls    int
}

func (s *musicScannerSpotifyStub) SearchArtistByName(_ context.Context, name string) (*spotifylib.FullArtist, error) {
	s.artistCalls++
	if s.artistsByName != nil {
		artist, ok := s.artistsByName[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, errors.New("no spotify match")
		}
		return artist, nil
	}
	if s.artistErr != nil {
		return nil, s.artistErr
	}

	return s.artist, nil
}

func (s *musicScannerSpotifyStub) SearchAndGetAlbumDetails(_ context.Context, _, _ string) (*spotifylib.FullAlbum, error) {
	s.albumCalls++
	if s.albumErr != nil {
		return nil, s.albumErr
	}

	return s.album, nil
}

func (s *musicScannerSpotifyStub) SearchAlbums(_ context.Context, _ string) ([]spotifylib.SimpleAlbum, error) {
	return nil, nil
}

func (s *musicScannerSpotifyStub) SearchTracks(_ context.Context, _ string) ([]spotifylib.FullTrack, error) {
	return nil, nil
}

func (s *musicScannerSpotifyStub) ClearAllCaches() {
	s.clearCalls++
}

// scan runs one complete library scan synchronously, publishing the run the
// way Start does.
func (s *Scanner) scan(directory string) {
	s.beginReport()
	s.runMusicScan(directory)
}

func runMusicScanForTest(t *testing.T, app *Scanner) {
	t.Helper()

	app.scan(app.currentMusicDirectory().String)
}

// processBatchReport runs one batch against a fresh report.
func (s *Scanner) processBatchReport(ctx context.Context, scan *musicScanContext, files []scanner.ScanFile) *scanReport {
	report := newScanReport(Status{})
	s.processMusicBatch(ctx, scan, report, files)
	return report
}

// processBatchCounts returns the scanned (imported or updated), skipped and
// failed counts one batch recorded.
func (s *Scanner) processBatchCounts(ctx context.Context, scan *musicScanContext, files []scanner.ScanFile) (scanned, skipped, failures int) {
	report := s.processBatchReport(ctx, scan, files)
	return report.status.Imported + report.status.Updated, report.status.Unchanged, report.status.Failed
}

func writeMusicScannerTestFile(t *testing.T, path, contents string) int64 {
	t.Helper()

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("create test music directory: %v", err)
	}

	err = os.WriteFile(path, []byte(contents), 0644)
	if err != nil {
		t.Fatalf("write test music file: %v", err)
	}

	return int64(len(contents))
}

// Metadata tests use synthetic media bytes with stubbed ffprobe responses.
// Resize only when a fixture requests a content change; repeated calls preserve
// timestamps so they exercise the actual unchanged-file path.
func prepareMusicFixtures(t testing.TB, files []scanner.ScanFile) {
	t.Helper()
	for _, file := range files {
		info, err := os.Stat(file.Path)
		if err == nil && info.Size() == file.Size {
			continue
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		err = os.MkdirAll(filepath.Dir(file.Path), 0755)
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.OpenFile(file.Path, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}
		err = f.Truncate(file.Size)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatal(errors.Join(err, closeErr))
		}
	}
}

func (s *Scanner) processMusicFixtureBatch(t testing.TB, ctx context.Context, scan *musicScanContext, files []scanner.ScanFile) (int, int, int) {
	t.Helper()
	prepareMusicFixtures(t, files)
	return s.processBatchCounts(ctx, scan, files)
}
