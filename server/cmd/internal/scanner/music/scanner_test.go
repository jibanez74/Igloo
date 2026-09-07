package music

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/scanner"
	"igloo/sqlc"

	_ "github.com/mattn/go-sqlite3"
	spotifylib "github.com/zmb3/spotify/v2"
)

type capturedLogEntry struct {
	msg  string
	args []any
}

type capturedLogger struct {
	mu           sync.Mutex
	debugEntries []capturedLogEntry
	infoEntries  []capturedLogEntry
	warnEntries  []capturedLogEntry
	errorEntries []capturedLogEntry
}

func (l *capturedLogger) log(entries *[]capturedLogEntry, msg string, args []any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := capturedLogEntry{msg: msg, args: append([]any(nil), args...)}
	*entries = append(*entries, entry)
}

func (l *capturedLogger) Debug(msg string, args ...any) { l.log(&l.debugEntries, msg, args) }
func (l *capturedLogger) Info(msg string, args ...any)  { l.log(&l.infoEntries, msg, args) }
func (l *capturedLogger) Warn(msg string, args ...any)  { l.log(&l.warnEntries, msg, args) }
func (l *capturedLogger) Error(msg string, args ...any) { l.log(&l.errorEntries, msg, args) }

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
	return New(Dependencies{DB: db, Queries: queries, Logger: &capturedLogger{}})
}

func countScannerRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()

	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		t.Fatalf("count rows: %v", err)
	}

	return count
}

// noKeyframeProbe completes ffprobe.FfprobeInterface for music scanner stubs,
// which never serve HLS.
type noKeyframeProbe struct{}

func (noKeyframeProbe) KeyframeAtOrBefore(context.Context, string, int64, float64) (float64, error) {
	return 0, errors.New("keyframe probing is not stubbed")
}

func (app *Scanner) processMusicBatchForTest(ctx context.Context, files []scanner.ScanFile) (scanned, skipped, errCount int) {
	scanIndex, err := app.loadMusicScanIndex(ctx)
	if err != nil {
		app.logger.Error(fmt.Sprintf("failed to load music scan index: %s", err.Error()))
		return 0, 0, len(files)
	}

	return app.processMusicBatch(ctx, newMusicScanContext(scanIndex), files)
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
	noKeyframeProbe
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
	noKeyframeProbe
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
	noKeyframeProbe
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
	noKeyframeProbe
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
	artist      *spotifylib.FullArtist
	artistErr   error
	artistCalls int
	album       *spotifylib.FullAlbum
	albumErr    error
	albumCalls  int
	clearCalls  int
}

func (s *musicScannerSpotifyStub) SearchArtistByName(_ context.Context, _ string) (*spotifylib.FullArtist, error) {
	s.artistCalls++
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

func runMusicScanForTest(t *testing.T, app *Scanner) {
	t.Helper()

	app.runMusicScan(app.currentMusicDirectory().String)
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
