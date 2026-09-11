package movie

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

func setupMovieScanner(t *testing.T) *movieScannerTestContext {
	t.Helper()
	return setupMovieScannerDatabase(t, ":memory:?_foreign_keys=on")
}

func setupMovieScannerDatabase(t *testing.T, source string) *movieScannerTestContext {
	t.Helper()
	db, err := sql.Open("sqlite3", source)
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
		Now:         func() time.Time { return time.Now().Add(2 * time.Minute) },
		DB:          db,
		Queries:     queries,
		Logger:      &scannertest.Logger{},
		ScanContext: context.Background(),
		ScannerDBMu: &sync.Mutex{},
		CurrentMoviesDirectory: func() sql.NullString {
			return ctx.moviesDir
		},
	})
	return ctx
}

type stubMovieScannerFfprobe struct {
	mu sync.Mutex
	scannertest.NoKeyframeProbe
	result  *ffprobe.FfprobeResult
	results []*ffprobe.FfprobeResult
	calls   int
}

func (s *stubMovieScannerFfprobe) GetMetadata(_ context.Context, filePath string) (*ffprobe.FfprobeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

type stubMovieScannerTmdb struct {
	mu            sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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

// scan runs one complete library scan synchronously, publishing the run the
// way Start does before handing off to the scan goroutine.
func (s *Scanner) scan(directory string) {
	s.beginReport()
	s.runMovieScan(directory)
}

// enrichmentAttempted mirrors the pipeline's one-lookup-per-scan guarantee,
// which enrichMovies gets structurally by running once, for fixtures that push
// the same file through processFile several times on one scan context.
var enrichmentAttempted = map[*movieScanContext]map[string]bool{}

// These focused persistence fixtures execute both phases for one file. Full
// pipeline tests below exercise scheduling, accounting, and phase separation.
func (s *Scanner) processFile(ctx context.Context, scan *movieScanContext, file scanner.ScanFile) (scanner.FileOutcome, error) {
	file.Path = filepath.Clean(file.Path)
	result := s.prepareFile(ctx, probeJob{file: file, baseline: scan.movieIndex[file.Path]})
	if result.inspection != nil {
		defer result.inspection.Close()
	}
	if result.err != nil {
		return scanner.FileNeedsProcessing, result.err
	}
	outcome := result.inspection.Outcome
	if outcome == scanner.FileDeferred {
		return outcome, nil
	}
	if result.resolved != nil {
		err := s.persistLocalMovie(ctx, scan, result.resolved)
		if err != nil {
			return outcome, err
		}
	}
	baseline := scan.movieIndex[file.Path]
	if s.tmdb == nil || !baseline.enrichmentEligible(s.now()) || enrichmentAttempted[scan][file.Path] {
		return outcome, nil
	}
	if enrichmentAttempted[scan] == nil {
		enrichmentAttempted[scan] = map[string]bool{}
	}
	enrichmentAttempted[scan][file.Path] = true
	enriched := s.prepareEnrichment(ctx, enrichmentJob{file: file, baseline: baseline})
	if enriched.inspection != nil {
		defer enriched.inspection.Close()
	}
	if enriched.err != nil {
		if ctx.Err() != nil {
			return outcome, ctx.Err()
		}
		return outcome, nil
	}
	_, err := s.persistEnrichment(ctx, scan, enriched.resolved)
	return outcome, err
}

func (s *Scanner) processMoviesBatch(ctx context.Context, scan *movieScanContext, files []scanner.ScanFile) (scanned, skipped, failures, deferred int) {
	for _, file := range files {
		outcome, err := s.processFile(ctx, scan, file)
		if ctx.Err() != nil {
			return
		}
		var deferral *scanner.FileDeferral
		isDeferred := errors.As(err, &deferral)
		if isDeferred || (err == nil && outcome == scanner.FileDeferred) {
			deferred++
		} else if err != nil {
			failures++
		} else if outcome == scanner.FileUnchanged {
			skipped++
		} else {
			scanned++
		}
	}
	return
}

func (*stubMovieScannerTmdb) SearchShowsByTitleAndYear(context.Context, string, ...int) ([]tmdb.TVShow, error) {
	return nil, tmdb.ErrNoShowsFound
}
func (*stubMovieScannerTmdb) GetShowDetails(context.Context, int) (*tmdb.TVShow, error) {
	return nil, tmdb.ErrNoShowsFound
}
func (*stubMovieScannerTmdb) GetSeasonDetails(context.Context, int, int) (*tmdb.TVSeason, error) {
	return nil, tmdb.ErrNoShowsFound
}
func (*stubMovieScannerTmdb) GetEpisodeCredits(context.Context, int, int, int) (*tmdb.TVEpisodeCredits, error) {
	return nil, tmdb.ErrNoShowsFound
}
