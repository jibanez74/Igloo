package movie

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/tmdb"
	"igloo/sqlc"

	_ "github.com/mattn/go-sqlite3"
)

type movieScannerTestContext struct {
	db        *sql.DB
	queries   *database.Queries
	scanner   *Scanner
	moviesDir sql.NullString
}

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

func setupMovieScanner(t *testing.T) *movieScannerTestContext {
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

	ctx := &movieScannerTestContext{db: db, queries: queries}
	ctx.scanner = New(Dependencies{
		DB:          db,
		Queries:     queries,
		Logger:      &capturedLogger{},
		ScanContext: context.Background(),
		ScannerDBMu: &sync.Mutex{},
		CurrentMoviesDirectory: func() sql.NullString {
			return ctx.moviesDir
		},
	})
	return ctx
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

type stubMovieScannerFfprobe struct {
	noKeyframeProbe
	result  *ffprobe.FfprobeResult
	results []*ffprobe.FfprobeResult
	calls   int
}

func (s *stubMovieScannerFfprobe) GetMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	callIndex := s.calls
	s.calls++
	if callIndex < len(s.results) && s.results[callIndex] != nil {
		return s.results[callIndex], nil
	}
	return s.result, nil
}

func (s *stubMovieScannerFfprobe) GetAudioMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	return s.GetMetadata(context.Background(), filePath)
}

// noKeyframeProbe completes ffprobe.FfprobeInterface for stubs that only
// exercise scanning. Keyframe lookup is advisory on the playback path, so a
// stub that never serves HLS refuses it rather than inventing an offset.
type noKeyframeProbe struct{}

func (noKeyframeProbe) KeyframeAtOrBefore(
	_ context.Context,
	_ string,
	_ int64,
	_ float64,
) (float64, error) {
	return 0, errors.New("keyframe probing is not stubbed")
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

func movieScannerMetadataFixture(duration string) *ffprobe.FfprobeResult {
	return &ffprobe.FfprobeResult{
		Format: ffprobe.Format{
			Duration: duration,
		},
		Streams: []ffprobe.Stream{
			{
				Index:        0,
				CodecName:    "h264",
				CodecType:    "video",
				Profile:      "High",
				BitRate:      "5000000",
				Width:        1920,
				Height:       1080,
				CodedWidth:   1920,
				CodedHeight:  1080,
				AspectRatio:  "16:9",
				Level:        41,
				FrameRate:    "24000/1001",
				AvgFrameRate: "24000/1001",
				BitDepth:     "8",
				PixelFormat:  "yuv420p",
				Tags: ffprobe.StreamTags{
					Language: "eng",
					Title:    "Main Video",
				},
			},
			{
				Index:       1,
				CodecName:   "mjpeg",
				CodecType:   "video",
				Width:       600,
				Height:      900,
				Disposition: ffprobe.StreamDisposition{AttachedPic: 1},
			},
			{
				Index:         2,
				CodecName:     "aac",
				CodecType:     "audio",
				Profile:       "LC",
				BitRate:       "192000",
				SampleRate:    "48000",
				Channels:      6,
				ChannelLayout: "5.1",
				Tags: ffprobe.StreamTags{
					Language: "eng",
					Title:    "Surround",
				},
			},
			{
				Index:     3,
				CodecName: "subrip",
				CodecType: "subtitle",
				Tags: ffprobe.StreamTags{
					Language: "eng",
					Title:    "English",
				},
			},
		},
		Chapters: []ffprobe.Chapter{
			{StartTime: "0.000000", Tags: ffprobe.ChapterTags{Title: "Opening"}},
			{StartTime: "120.500000", Tags: ffprobe.ChapterTags{Title: "Follow the White Rabbit"}},
		},
	}
}

func tmdbMovieFromJSON(t *testing.T, payload string) tmdb.TmdbMovie {
	t.Helper()

	var movie tmdb.TmdbMovie
	err := json.Unmarshal([]byte(payload), &movie)
	if err != nil {
		t.Fatalf("unmarshal tmdb fixture: %v", err)
	}
	return movie
}
