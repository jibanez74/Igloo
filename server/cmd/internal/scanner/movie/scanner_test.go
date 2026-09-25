package movie

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/tmdb"

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
	db, queries := scannertest.OpenDB(t, source)

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

// stubMovieScannerTmdb answers searches and detail lookups from fixtures and
// records every call. searchHook, when set, answers a search after the call
// is recorded; detailHook runs before a detail lookup is answered.
type stubMovieScannerTmdb struct {
	tmdb.TmdbInterface
	mu            sync.Mutex
	searchErr     error
	detailErr     error
	searchResults []tmdb.TmdbMovie
	detailMovies  map[int]tmdb.TmdbMovie
	searchCalls   []stubMovieScannerTmdbSearchCall
	detailCalls   []int
	searchHook    func(ctx context.Context, title string, year int) ([]tmdb.TmdbMovie, error)
	detailHook    func()
}

type stubMovieScannerTmdbSearchCall struct {
	title string
	year  []int
}

func (s *stubMovieScannerTmdb) GetTmdbMovieByID(_ context.Context, movie *tmdb.TmdbMovie) error {
	if s.detailHook != nil {
		s.detailHook()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.detailCalls = append(s.detailCalls, movie.TmdbID)
	if s.detailErr != nil {
		return s.detailErr
	}
	details, ok := s.detailMovies[movie.TmdbID]
	if !ok {
		return errors.New("tmdb details unavailable")
	}
	*movie = details
	return nil
}

func (s *stubMovieScannerTmdb) SearchMoviesByTitleAndYear(ctx context.Context, title string, year ...int) ([]tmdb.TmdbMovie, error) {
	s.mu.Lock()
	yearCopy := append([]int(nil), year...)
	s.searchCalls = append(s.searchCalls, stubMovieScannerTmdbSearchCall{title: title, year: yearCopy})
	hook := s.searchHook
	s.mu.Unlock()
	if hook != nil {
		firstYear := 0
		if len(year) > 0 {
			firstYear = year[0]
		}
		return hook(ctx, title, firstYear)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	results := make([]tmdb.TmdbMovie, len(s.searchResults))
	copy(results, s.searchResults)
	return results, nil
}

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

// processBatchReport runs files through the production local and enrichment
// phases on one scan context and returns the report they filled in. Focused
// persistence tests build on it; the pipeline tests exercise scheduling,
// accounting, and phase separation through scan.
func (s *Scanner) processBatchReport(ctx context.Context, scan *movieScanContext, files []scanner.ScanFile) *scanReport {
	report := newScanReport(Status{})
	report.scan = scan
	local := make([]localFile, len(files))
	indexes := make([]int, len(files))
	for i, file := range files {
		file.Path = filepath.Clean(file.Path)
		local[i] = localFile{file: file}
		indexes[i] = i
	}
	s.processLocal(ctx, scan, report, local, indexes, time.Time{})
	if ctx.Err() == nil {
		s.enrichMovies(ctx, scan, report, local)
	}
	return report
}

func (s *Scanner) processMoviesBatch(ctx context.Context, scan *movieScanContext, files []scanner.ScanFile) (scanned, skipped, failures, deferred int) {
	report := s.processBatchReport(ctx, scan, files)
	return report.status.Imported + report.status.Updated, report.status.Unchanged, report.status.Failed, report.status.Deferred
}

// processFile runs one file through both phases and reports its local outcome
// plus the first failure the pipeline recorded for it, or the cancellation.
func (s *Scanner) processFile(ctx context.Context, scan *movieScanContext, file scanner.ScanFile) (scanner.FileOutcome, error) {
	report := s.processBatchReport(ctx, scan, []scanner.ScanFile{file})
	contextErr := ctx.Err()
	if contextErr != nil {
		return scanner.FileNeedsProcessing, contextErr
	}
	outcome := scanner.FileNeedsProcessing
	if report.status.Unchanged == 1 {
		outcome = scanner.FileUnchanged
	}
	if report.status.Deferred == 1 {
		outcome = scanner.FileDeferred
	}
	if report.status.Failed+report.status.EnrichmentFailed > 0 {
		return outcome, fmt.Errorf("%s: %w", report.status.Issues[0].Reason, s.lastLoggedFailure())
	}
	return outcome, nil
}

// lastLoggedFailure returns the error the pipeline logged for the file it just
// processed, so callers can match the cause behind the user-facing reason.
func (s *Scanner) lastLoggedFailure() error {
	log, ok := s.logger.(*scannertest.Logger)
	if !ok {
		return errors.New("failure not captured")
	}
	for _, entries := range [][]scannertest.LogEntry{log.WarnEntries, log.ErrorEntries} {
		for i := len(entries) - 1; i >= 0; i-- {
			for j := 0; j+1 < len(entries[i].Args); j += 2 {
				err, isError := entries[i].Args[j+1].(error)
				if entries[i].Args[j] == "error" && isError {
					return err
				}
			}
		}
	}
	return errors.New("failure not logged")
}
