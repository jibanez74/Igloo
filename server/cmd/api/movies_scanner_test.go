package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/tmdb"
)

type stubMovieScannerFfprobe struct {
	noKeyframeProbe
	result  *ffprobe.FfprobeResult
	results []*ffprobe.FfprobeResult
	calls   int
}

func (s *stubMovieScannerFfprobe) GetMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	callIndex := s.calls
	s.calls++
	if callIndex < len(s.results) && s.results[callIndex] != nil {
		return s.results[callIndex], nil
	}
	return s.result, nil
}

func (s *stubMovieScannerFfprobe) GetAudioMetadata(filePath string) (*ffprobe.FfprobeResult, error) {
	return s.GetMetadata(filePath)
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

func TestProcessMoviesBatchSkipsUnchangedWithoutFfprobe(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Unchanged.Movie.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	ffprobeStub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	app.Ffprobe = ffprobeStub

	scan := newMovieScanContext(map[string]int64{path: 5})
	scanned, skipped, errCount := app.processMoviesBatch(context.Background(), scan, []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})

	if scanned != 0 || skipped != 1 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/1/0", scanned, skipped, errCount)
	}
	if ffprobeStub.calls != 0 {
		t.Fatalf("expected unchanged movie to skip ffprobe, got %d calls", ffprobeStub.calls)
	}
}

func TestProcessMoviesBatchRollsBackInvalidMovieFile(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Audio.Only.2020.mkv")
	err := os.WriteFile(path, []byte("movie"), 0o644)
	if err != nil {
		t.Fatalf("write movie: %v", err)
	}

	app.Ffprobe = &stubMovieScannerFfprobe{
		result: &ffprobe.FfprobeResult{
			Format: ffprobe.Format{
				Duration: "120",
			},
			Streams: []ffprobe.Stream{
				{
					Index:     0,
					CodecName: "aac",
					CodecType: "audio",
					Channels:  2,
				},
			},
		},
	}
	app.Tmdb = &stubMovieScannerTmdb{searchErr: errors.New("tmdb unavailable")}

	scanned, skipped, errCount := app.processMoviesBatch(context.Background(), newMovieScanContext(nil), []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})

	if scanned != 0 || skipped != 0 || errCount != 1 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 0/0/1", scanned, skipped, errCount)
	}
	if got := countScannerRows(t, app.DB, "SELECT COUNT(*) FROM movies WHERE file_path = ?", path); got != 0 {
		t.Fatalf("expected invalid movie transaction to roll back, got %d movie rows", got)
	}
}

func TestRunMovieScanPreservesMissingMovieRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	missingPath := filepath.Join(moviesDir, "Missing.Movie.1999.mkv")
	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Missing Movie",
		FilePath:  missingPath,
		FileName:  filepath.Base(missingPath),
		Size:      7,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("insert missing movie: %v", err)
	}

	app.Settings = &database.Setting{MoviesDir: sql.NullString{String: moviesDir, Valid: true}}
	app.runMovieScan()

	_, err = app.Queries.GetMovieByID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("expected missing movie row to be preserved: %v", err)
	}
}

func TestRunMovieScan_AcceptsConfiguredVideoExtensions(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	files := []struct {
		path string
		ext  string
	}{
		{path: filepath.Join(moviesDir, "Sample Movie (2020).mov"), ext: "mov"},
		{path: filepath.Join(moviesDir, "Sample Movie (2021).m4v"), ext: "m4v"},
		{path: filepath.Join(moviesDir, "Sample Movie (2022).webm"), ext: "webm"},
	}
	for _, file := range files {
		err := os.WriteFile(file.path, []byte("movie"), 0o644)
		if err != nil {
			t.Fatalf("write movie %s: %v", file.path, err)
		}
	}

	ffprobeStub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	app.Ffprobe = ffprobeStub
	app.Settings = &database.Setting{MoviesDir: sql.NullString{String: moviesDir, Valid: true}}

	app.runMovieScan()

	if ffprobeStub.calls != len(files) {
		t.Fatalf("ffprobe calls = %d, want %d", ffprobeStub.calls, len(files))
	}

	for _, file := range files {
		var container string
		err := app.DB.QueryRowContext(context.Background(), `
			SELECT container
			FROM movies
			WHERE file_path = ?
			LIMIT 1
		`, file.path).Scan(&container)
		if err != nil {
			t.Fatalf("get movie %s: %v", file.path, err)
		}
		if container != file.ext {
			t.Fatalf("movie container = %q, want %q", container, file.ext)
		}
	}
}

func TestResolveMovieFilePinsMimeTypePerContainer(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.Ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}

	// Expected values are literals on purpose: a bad edit to
	// helpers.VideoMimeTypes must fail here, so do not assert against the map.
	cases := []struct {
		ext  string
		want string
	}{
		{ext: "mp4", want: "video/mp4"},
		{ext: "m4v", want: "video/mp4"},
		{ext: "mkv", want: "video/x-matroska"},
		{ext: "webm", want: "video/webm"},
		{ext: "avi", want: "video/x-msvideo"},
		{ext: "mov", want: "video/quicktime"},
	}

	for _, tc := range cases {
		resolved, err := app.resolveMovieFile(context.Background(), helpers.ScanFile{
			Path: "/movies/Sample.Movie.2024." + tc.ext,
			Ext:  tc.ext,
			Size: 100,
		})
		if err != nil {
			t.Fatalf("resolve %s movie: %v", tc.ext, err)
		}
		if resolved.params.MimeType != tc.want {
			t.Errorf("mime_type for .%s = %q, want %q", tc.ext, resolved.params.MimeType, tc.want)
		}
		if resolved.params.Container != tc.ext {
			t.Errorf("container for .%s = %q, want %q", tc.ext, resolved.params.Container, tc.ext)
		}
	}
}

func TestProcessMovieStreamsPersistsDispositions(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Disposition Movie",
		FilePath:  "/movies/Disposition.Movie.2024.mp4",
		FileName:  "Disposition.Movie.2024.mp4",
		Size:      1024,
		Container: "mp4",
		MimeType:  helpers.VideoMimeTypes["mp4"],
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}

	fixture := movieScannerMetadataFixture("120")
	fixture.Streams = append(fixture.Streams,
		ffprobe.Stream{
			Index:       4,
			CodecName:   "aac",
			CodecType:   "audio",
			Channels:    2,
			Tags:        ffprobe.StreamTags{Language: "eng", Title: "Main"},
			Disposition: ffprobe.StreamDisposition{Default: 1},
		},
		ffprobe.Stream{
			Index:       5,
			CodecName:   "subrip",
			CodecType:   "subtitle",
			Tags:        ffprobe.StreamTags{Language: "eng", Title: "Signs"},
			Disposition: ffprobe.StreamDisposition{Forced: 1, Default: 1},
		},
	)

	_, err = app.processMovieStreams(ctx, app.Queries, movie.ID, fixture.Streams)
	if err != nil {
		t.Fatalf("process movie streams: %v", err)
	}

	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get audio streams: %v", err)
	}
	if len(audioStreams) != 2 {
		t.Fatalf("audio stream count = %d, want 2", len(audioStreams))
	}
	if audioStreams[0].IsDefault {
		t.Error("first audio stream (no disposition) persisted is_default=true, want false")
	}
	if !audioStreams[1].IsDefault {
		t.Error("default-flagged audio stream persisted is_default=false, want true")
	}

	subtitles, err := app.Queries.GetSubtitlesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get subtitles: %v", err)
	}
	if len(subtitles) != 2 {
		t.Fatalf("subtitle count = %d, want 2", len(subtitles))
	}
	if subtitles[0].IsForced || subtitles[0].IsDefault {
		t.Error("plain subtitle persisted disposition flags, want none")
	}
	if !subtitles[1].IsForced || !subtitles[1].IsDefault {
		t.Error("forced+default subtitle lost its flags")
	}
}

// field_order and the display-matrix rotation feed the deinterlace and remux
// decisions, so the scanner must persist them faithfully — including the
// difference between an explicit 0-degree matrix and no matrix at all.
func TestProcessMovieStreamsPersistsFieldOrderAndRotation(t *testing.T) {
	tests := []struct {
		name         string
		fieldOrder   string
		sideData     []ffprobe.StreamSideData
		wantOrder    sql.NullString
		wantRotation sql.NullInt64
	}{
		{
			name:         "interlaced with rotation",
			fieldOrder:   "tt",
			sideData:     []ffprobe.StreamSideData{{SideDataType: "Display Matrix", Rotation: -90}},
			wantOrder:    sql.NullString{String: "tt", Valid: true},
			wantRotation: sql.NullInt64{Int64: -90, Valid: true},
		},
		{
			name:         "explicit zero-degree matrix stays distinguishable",
			fieldOrder:   "progressive",
			sideData:     []ffprobe.StreamSideData{{SideDataType: "Display Matrix", Rotation: 0}},
			wantOrder:    sql.NullString{String: "progressive", Valid: true},
			wantRotation: sql.NullInt64{Int64: 0, Valid: true},
		},
		{
			name: "absent metadata persists as NULL",
		},
		{
			name:     "non-matrix side data carries no rotation",
			sideData: []ffprobe.StreamSideData{{SideDataType: "H.26[45] User Data Unregistered SEI message"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(t)
			defer app.DB.Close()
			ctx := context.Background()

			movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
				Title:     "Field Order Movie",
				FilePath:  "/movies/Field.Order.Movie.2024.mp4",
				FileName:  "Field.Order.Movie.2024.mp4",
				Size:      1024,
				Container: "mp4",
				MimeType:  helpers.VideoMimeTypes["mp4"],
			})
			if err != nil {
				t.Fatalf("insert movie: %v", err)
			}

			fixture := movieScannerMetadataFixture("120")
			fixture.Streams[0].FieldOrder = tt.fieldOrder
			fixture.Streams[0].SideDataList = tt.sideData

			_, err = app.processMovieStreams(ctx, app.Queries, movie.ID, fixture.Streams)
			if err != nil {
				t.Fatalf("process movie streams: %v", err)
			}

			videoStreams, err := app.Queries.GetVideoStreamsByMovieID(ctx, movie.ID)
			if err != nil {
				t.Fatalf("get video streams: %v", err)
			}
			if len(videoStreams) == 0 {
				t.Fatal("no video streams persisted")
			}
			if videoStreams[0].FieldOrder != tt.wantOrder {
				t.Errorf("field_order = %+v, want %+v", videoStreams[0].FieldOrder, tt.wantOrder)
			}
			if videoStreams[0].Rotation != tt.wantRotation {
				t.Errorf("rotation = %+v, want %+v", videoStreams[0].Rotation, tt.wantRotation)
			}
		})
	}
}

func TestMovieScannerUpsertPreservesAudienceRatingAndRefreshesMetadata(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	path := "/movies/Moneyball.2011.mkv"

	_, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball",
		FilePath:  path,
		FileName:  "Moneyball.2011.mkv",
		Size:      100,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Adult:     false,
		Overview:  helpers.NullString("Original overview"),
		AudienceRating: sql.NullFloat64{
			Float64: 8.7,
			Valid:   true,
		},
	})
	if err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	updated, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Moneyball Remastered",
		FilePath:  path,
		FileName:  "Moneyball.2011.mkv",
		Size:      200,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Adult:     false,
		Overview:  helpers.NullString("Scanner overview"),
	})
	if err != nil {
		t.Fatalf("scanner upsert: %v", err)
	}

	if updated.Title != "Moneyball Remastered" {
		t.Fatalf("expected scanner title to overwrite manual title, got %q", updated.Title)
	}
	if !updated.Overview.Valid || updated.Overview.String != "Scanner overview" {
		t.Fatalf("expected scanner overview to overwrite manual overview, got %+v", updated.Overview)
	}
	if updated.Size != 200 {
		t.Fatalf("expected scanner-owned size to update to 200, got %d", updated.Size)
	}
	if !updated.AudienceRating.Valid || updated.AudienceRating.Float64 != 8.7 {
		t.Fatalf("expected audience rating to remain 8.7, got %+v", updated.AudienceRating)
	}
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

func TestSelectBestTmdbMatch(t *testing.T) {
	t.Run("empty results returns nil", func(t *testing.T) {
		results := []tmdb.TmdbMovie{}
		result := selectBestTmdbMatch(results, "test movie", 2023)
		if result != nil {
			t.Errorf("Expected nil for empty results, got %v", result)
		}
	})

	t.Run("single result returns that result", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Test Movie",
				ReleaseDate: "2023-01-01",
				Popularity:  50.0,
				VoteAverage: 7.5,
			},
		}
		result := selectBestTmdbMatch(results, "Test Movie", 2023)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1, got %d", result.Movie.TmdbID)
		}
	})

	t.Run("clean title beats noisy similar candidate", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Moneyball",
				ReleaseDate: "2011-09-22",
				Popularity:  35.0,
				VoteAverage: 7.6,
			},
			{
				TmdbID:      2,
				Title:       "Balls of Fury",
				ReleaseDate: "2007-08-29",
				Popularity:  50.0,
				VoteAverage: 7.0,
			},
		}
		result := selectBestTmdbMatch(results, "Moneyball", 2011)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1 (best title match), got %d", result.Movie.TmdbID)
		}
	})

	t.Run("missing year still chooses strongest title match", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Train Dreams",
				ReleaseDate: "",
				Popularity:  5.0,
				VoteAverage: 6.0,
			},
			{
				TmdbID:      2,
				Title:       "Dream Scenario",
				ReleaseDate: "2023-01-01",
				Popularity:  20.0,
				VoteAverage: 7.0,
			},
		}
		result := selectBestTmdbMatch(results, "Train Dreams", 2025)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Movie.TmdbID != 1 {
			t.Errorf("Expected TMDB ID 1 (best title match), got %d", result.Movie.TmdbID)
		}
	})

	t.Run("confidence stays bounded", func(t *testing.T) {
		results := []tmdb.TmdbMovie{
			{
				TmdbID:      1,
				Title:       "Goldfinger",
				ReleaseDate: "1964-09-20",
				Popularity:  200.0,
				VoteAverage: 10.0,
			},
		}
		result := selectBestTmdbMatch(results, "Goldfinger", 1964)
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.Confidence < 0 || result.Confidence > 100 {
			t.Errorf("Expected bounded confidence, got %f", result.Confidence)
		}
	})
}

func TestRankTmdbMatches_SortsBestCandidateFirst(t *testing.T) {
	results := []tmdb.TmdbMovie{
		{
			TmdbID:      1,
			Title:       "Casino Royale",
			ReleaseDate: "1967-04-13",
			Popularity:  40.0,
			VoteAverage: 6.1,
		},
		{
			TmdbID:      2,
			Title:       "Casino Royale",
			ReleaseDate: "2006-11-14",
			Popularity:  35.0,
			VoteAverage: 7.6,
		},
		{
			TmdbID:      3,
			Title:       "Quantum of Solace",
			ReleaseDate: "2008-10-29",
			Popularity:  50.0,
			VoteAverage: 6.3,
		},
	}

	ranked := rankTmdbMatches(results, "Casino Royale", 2006)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 ranked results, got %d", len(ranked))
	}
	if ranked[0].Movie.TmdbID != 2 {
		t.Fatalf("expected 2006 Casino Royale first, got TMDB ID %d", ranked[0].Movie.TmdbID)
	}
	if ranked[1].Movie.TmdbID != 1 {
		t.Fatalf("expected 1967 Casino Royale second, got TMDB ID %d", ranked[1].Movie.TmdbID)
	}
}

func TestNormalizeMovieTitleForSearch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{
			input: "Moneyball.2011.REMASTERED.2160p.4K.WEB.x265.10bit.AAC5.1-[YTS.MX]",
			want:  "moneyball 2011",
		},
		{
			input: "Mary.Queen.of.Scots",
			want:  "mary queen of scots",
		},
		{
			input: "If.I.Had.Legs.Id.Kick.You",
			want:  "if i had legs id kick you",
		},
	}

	for _, tt := range tests {
		got := normalizeMovieTitleForSearch(tt.input)
		if got != tt.want {
			t.Errorf("normalizeMovieTitleForSearch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestProcessMoviesBatchWithTmdbPersistsMetadataRelationshipsAndStreams(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "The.Matrix.1999.mkv")
	if err := os.WriteFile(path, []byte("movie"), 0o644); err != nil {
		t.Fatalf("write movie: %v", err)
	}

	tmdbDetails := tmdbMovieFromJSON(t, `{
		"id": 603,
		"title": "The Matrix",
		"original_title": "The Matrix",
		"overview": "A computer hacker learns about the true nature of reality.",
		"release_date": "1999-03-31",
		"poster_path": "/matrix-poster.jpg",
		"backdrop_path": "/matrix-backdrop.jpg",
		"vote_average": 8.2,
		"adult": false,
		"original_language": "en",
		"runtime": 136,
		"tagline": "Welcome to the Real World.",
		"budget": 63000000,
		"revenue": 463517383,
		"imdb_id": "tt0133093",
		"genres": [
			{"id": 28, "name": "Action"},
			{"id": 878, "name": "Science Fiction"}
		],
		"production_companies": [
			{"id": 174, "name": "Warner Bros.", "logo_path": "/wb.png", "origin_country": "US"}
		],
		"credits": {
			"cast": [
				{"id": 6384, "name": "Keanu Reeves", "character": "Neo", "profile_path": "/keanu.jpg", "order": 0}
			],
			"crew": [
				{"id": 9339, "name": "Lana Wachowski", "job": "Director", "department": "Directing", "profile_path": "/lana.jpg"}
			]
		},
		"videos": {
			"results": [
				{"id": "533ec654c3a36854480003eb", "key": "vKQi3bBA1y8", "name": "Official Trailer", "site": "YouTube", "type": "Trailer", "official": true}
			]
		},
		"release_dates": {
			"results": [
				{"iso_3166_1": "GB", "release_dates": [{"certification": "15"}]},
				{"iso_3166_1": "US", "release_dates": [{"certification": "R"}]}
			]
		}
	}`)

	tmdbStub := &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{{
			TmdbID:      603,
			Title:       "The Matrix",
			ReleaseDate: "1999-03-31",
			VoteAverage: 8.2,
		}},
		detailMovies: map[int]tmdb.TmdbMovie{603: tmdbDetails},
	}
	app.Tmdb = tmdbStub
	app.Ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("5432.4")}

	scanned, skipped, errCount := app.processMoviesBatch(ctx, newMovieScanContext(nil), []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}
	if len(tmdbStub.searchCalls) != 1 {
		t.Fatalf("expected 1 TMDB search, got %d", len(tmdbStub.searchCalls))
	}
	if tmdbStub.searchCalls[0].title != "the matrix" {
		t.Fatalf("TMDB search title = %q, want the matrix", tmdbStub.searchCalls[0].title)
	}
	if len(tmdbStub.searchCalls[0].year) != 1 || tmdbStub.searchCalls[0].year[0] != 1999 {
		t.Fatalf("TMDB search year = %#v, want [1999]", tmdbStub.searchCalls[0].year)
	}
	if len(tmdbStub.detailCalls) != 1 || tmdbStub.detailCalls[0] != 603 {
		t.Fatalf("TMDB detail calls = %#v, want [603]", tmdbStub.detailCalls)
	}

	movie := struct {
		ID            int64
		Title         string
		TmdbID        sql.NullInt64
		Size          int64
		Year          sql.NullInt64
		ReleaseDate   sql.NullString
		Certification sql.NullString
		Language      sql.NullString
		RunTime       sql.NullInt64
		Duration      sql.NullFloat64
	}{}
	err := app.DB.QueryRowContext(ctx, `
		SELECT id, title, tmdb_id, size, year, release_date, certification, language, run_time, duration
		FROM movies
		WHERE file_path = ?
		LIMIT 1
	`, path).Scan(
		&movie.ID,
		&movie.Title,
		&movie.TmdbID,
		&movie.Size,
		&movie.Year,
		&movie.ReleaseDate,
		&movie.Certification,
		&movie.Language,
		&movie.RunTime,
		&movie.Duration,
	)
	if err != nil {
		t.Fatalf("get movie by path: %v", err)
	}
	if movie.Title != "The Matrix" {
		t.Fatalf("movie title = %q, want The Matrix", movie.Title)
	}
	if !movie.TmdbID.Valid || movie.TmdbID.Int64 != 603 {
		t.Fatalf("tmdb id = %+v, want 603", movie.TmdbID)
	}
	if movie.Size != 5 {
		t.Fatalf("movie size = %d, want filesystem size 5", movie.Size)
	}
	if !movie.Year.Valid || movie.Year.Int64 != 1999 {
		t.Fatalf("year = %+v, want 1999", movie.Year)
	}
	if !movie.ReleaseDate.Valid || movie.ReleaseDate.String != "1999-03-31" {
		t.Fatalf("release date = %+v, want 1999-03-31", movie.ReleaseDate)
	}
	if !movie.Certification.Valid || movie.Certification.String != "R" {
		t.Fatalf("certification = %+v, want R", movie.Certification)
	}
	if !movie.Language.Valid || movie.Language.String != "en" {
		t.Fatalf("language = %+v, want en", movie.Language)
	}
	if !movie.RunTime.Valid || movie.RunTime.Int64 != 91 {
		t.Fatalf("runtime minutes = %+v, want 91", movie.RunTime)
	}
	if !movie.Duration.Valid || math.Abs(movie.Duration.Float64-5432.4) > 0.001 {
		t.Fatalf("duration = %+v, want 5432.4", movie.Duration)
	}

	genres, err := app.Queries.GetGenresByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	if got := movieGenreTags(genres); got != "Action,Science Fiction" {
		t.Fatalf("genres = %q, want Action,Science Fiction", got)
	}

	cast, err := app.Queries.GetCastByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get cast: %v", err)
	}
	if len(cast) != 1 || cast[0].ArtistName != "Keanu Reeves" || cast[0].Character != "Neo" {
		t.Fatalf("cast = %+v, want Keanu Reeves as Neo", cast)
	}

	crew, err := app.Queries.GetCrewByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get crew: %v", err)
	}
	if len(crew) != 1 || crew[0].ArtistName != "Lana Wachowski" || crew[0].Job != "Director" {
		t.Fatalf("crew = %+v, want Lana Wachowski Director", crew)
	}

	companies, err := app.Queries.GetProductionCompaniesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get production companies: %v", err)
	}
	if len(companies) != 1 || companies[0].Name != "Warner Bros." || companies[0].TmdbID != 174 {
		t.Fatalf("production companies = %+v, want Warner Bros.", companies)
	}

	extras, err := app.Queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extra videos: %v", err)
	}
	if len(extras) != 1 || extras[0].Title != "Official Trailer" || extras[0].Type != "trailer" || extras[0].Site != "youtube" {
		t.Fatalf("extra videos = %+v, want mapped YouTube trailer", extras)
	}

	videoStreams, err := app.Queries.GetVideoStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get video streams: %v", err)
	}
	if len(videoStreams) != 1 || videoStreams[0].StreamIndex != 0 || videoStreams[0].Codec != "h264" {
		t.Fatalf("video streams = %+v, want one h264 non-cover stream", videoStreams)
	}

	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get audio streams: %v", err)
	}
	if len(audioStreams) != 1 || audioStreams[0].StreamIndex != 2 || audioStreams[0].Channels != 6 {
		t.Fatalf("audio streams = %+v, want one 5.1 audio stream", audioStreams)
	}

	subtitles, err := app.Queries.GetSubtitlesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get subtitles: %v", err)
	}
	if len(subtitles) != 1 || subtitles[0].StreamIndex != 3 || subtitles[0].Codec != "subrip" {
		t.Fatalf("subtitles = %+v, want one subrip subtitle", subtitles)
	}

	chapters, err := app.Queries.GetChaptersByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get chapters: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Opening" || chapters[1].StartTime != 120 {
		t.Fatalf("chapters = %+v, want two normalized chapters", chapters)
	}
}

func TestProcessMoviesBatchWithTmdbReplacesScannerOwnedRelationshipsOnRescan(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	path := filepath.Join(moviesDir, "Replace.Me.2020.mkv")
	if err := os.WriteFile(path, []byte("movie"), 0o644); err != nil {
		t.Fatalf("write movie: %v", err)
	}

	firstDetails := tmdbMovieFromJSON(t, `{
		"id": 1000,
		"title": "Replace Me",
		"release_date": "2020-01-01",
		"genres": [{"id": 28, "name": "Action"}],
		"production_companies": [{"id": 1, "name": "Old Studio"}],
		"credits": {
			"cast": [{"id": 10, "name": "First Actor", "character": "Old Role", "order": 0}],
			"crew": [{"id": 11, "name": "First Director", "job": "Director", "department": "Directing"}]
		},
		"videos": {"results": [{"id": "old-video", "key": "old", "name": "Old Trailer", "site": "YouTube", "type": "Trailer"}]}
	}`)
	secondDetails := tmdbMovieFromJSON(t, `{
		"id": 1000,
		"title": "Replace Me Restored",
		"release_date": "2020-01-01",
		"genres": [{"id": 18, "name": "Drama"}],
		"production_companies": [{"id": 2, "name": "New Studio"}],
		"credits": {
			"cast": [{"id": 20, "name": "Second Actor", "character": "New Role", "order": 0}],
			"crew": [{"id": 21, "name": "Second Director", "job": "Director", "department": "Directing"}]
		},
		"videos": {"results": [{"id": "new-video", "key": "new", "name": "New Trailer", "site": "Vimeo", "type": "Featurette"}]}
	}`)
	tmdbStub := &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{{
			TmdbID:      1000,
			Title:       "Replace Me",
			ReleaseDate: "2020-01-01",
		}},
		detailMovies: map[int]tmdb.TmdbMovie{1000: firstDetails},
	}
	app.Tmdb = tmdbStub
	app.Ffprobe = &stubMovieScannerFfprobe{
		results: []*ffprobe.FfprobeResult{
			movieScannerMetadataFixture("120"),
			{
				Format: ffprobe.Format{Duration: "180"},
				Streams: []ffprobe.Stream{
					{Index: 4, CodecName: "hevc", CodecType: "video", Width: 3840, Height: 2160},
					{Index: 5, CodecName: "aac", CodecType: "audio", Channels: 2},
				},
				Chapters: []ffprobe.Chapter{
					{StartTime: "30.000000", Tags: ffprobe.ChapterTags{Title: "Only New Chapter"}},
				},
			},
		},
	}

	scan := newMovieScanContext(nil)
	scanned, skipped, errCount := app.processMoviesBatch(ctx, scan, []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	tmdbStub.detailMovies[1000] = secondDetails
	scanned, skipped, errCount = app.processMoviesBatch(ctx, scan, []helpers.ScanFile{
		{Path: path, Ext: "mkv", Size: 6},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("second scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	movie := struct {
		ID            int64
		Title         string
		TmdbID        sql.NullInt64
		Size          int64
		Year          sql.NullInt64
		ReleaseDate   sql.NullString
		Certification sql.NullString
		Language      sql.NullString
		RunTime       sql.NullInt64
		Duration      sql.NullFloat64
	}{}
	err := app.DB.QueryRowContext(ctx, `
		SELECT id, title, tmdb_id, size, year, release_date, certification, language, run_time, duration
		FROM movies
		WHERE file_path = ?
		LIMIT 1
	`, path).Scan(
		&movie.ID,
		&movie.Title,
		&movie.TmdbID,
		&movie.Size,
		&movie.Year,
		&movie.ReleaseDate,
		&movie.Certification,
		&movie.Language,
		&movie.RunTime,
		&movie.Duration,
	)
	if err != nil {
		t.Fatalf("get movie by path: %v", err)
	}
	if movie.Title != "Replace Me Restored" || movie.Size != 6 {
		t.Fatalf("movie after rescan = title %q size %d, want restored/6", movie.Title, movie.Size)
	}

	genres, err := app.Queries.GetGenresByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	if got := movieGenreTags(genres); got != "Drama" {
		t.Fatalf("genres after rescan = %q, want Drama", got)
	}

	cast, err := app.Queries.GetCastByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get cast: %v", err)
	}
	if len(cast) != 1 || cast[0].ArtistName != "Second Actor" || cast[0].Character != "New Role" {
		t.Fatalf("cast after rescan = %+v, want only second actor", cast)
	}

	crew, err := app.Queries.GetCrewByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get crew: %v", err)
	}
	if len(crew) != 1 || crew[0].ArtistName != "Second Director" {
		t.Fatalf("crew after rescan = %+v, want only second director", crew)
	}

	companies, err := app.Queries.GetProductionCompaniesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get production companies: %v", err)
	}
	if len(companies) != 1 || companies[0].Name != "New Studio" {
		t.Fatalf("production companies after rescan = %+v, want New Studio", companies)
	}

	extras, err := app.Queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extra videos: %v", err)
	}
	if len(extras) != 1 || extras[0].Title != "New Trailer" || extras[0].Type != "special_feature" || extras[0].Site != "vimeo" {
		t.Fatalf("extra videos after rescan = %+v, want mapped new featurette", extras)
	}

	videoStreams, err := app.Queries.GetVideoStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get video streams: %v", err)
	}
	if len(videoStreams) != 1 || videoStreams[0].StreamIndex != 4 || videoStreams[0].Codec != "hevc" {
		t.Fatalf("video streams after rescan = %+v, want one new hevc stream", videoStreams)
	}

	audioStreams, err := app.Queries.GetAudioStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get audio streams: %v", err)
	}
	if len(audioStreams) != 1 || audioStreams[0].StreamIndex != 5 {
		t.Fatalf("audio streams after rescan = %+v, want one new audio stream", audioStreams)
	}

	chapters, err := app.Queries.GetChaptersByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get chapters: %v", err)
	}
	if len(chapters) != 1 || chapters[0].Title != "Only New Chapter" || chapters[0].StartTime != 30 {
		t.Fatalf("chapters after rescan = %+v, want one new chapter", chapters)
	}
}

func TestMovieScannerEntityUpsertRefreshesMutableMetadata(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	movie, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     "Entity Cache",
		FilePath:  "/movies/Entity.Cache.2024.mkv",
		FileName:  "Entity.Cache.2024.mkv",
		Size:      100,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("upsert movie: %v", err)
	}

	firstCompanies := []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	}{
		{ID: 100, LogoPath: "/old-logo.png", Name: "Old Studio", OriginCountry: "US"},
	}
	if err := processProductionCompanies(ctx, app.Queries, movie.ID, firstCompanies); err != nil {
		t.Fatalf("process first production companies: %v", err)
	}

	firstVideos := []tmdb.TmdbVideoResult{
		{ID: "video-1", Key: "old-key", Name: "Old Trailer", Site: "YouTube", Type: "Trailer", Official: false},
	}
	if err := processExtraVideos(ctx, app.Queries, movie.ID, firstVideos); err != nil {
		t.Fatalf("process first extra videos: %v", err)
	}

	secondCompanies := []struct {
		ID            int    `json:"id"`
		LogoPath      string `json:"logo_path"`
		Name          string `json:"name"`
		OriginCountry string `json:"origin_country"`
	}{
		{ID: 100, LogoPath: "/new-logo.png", Name: "New Studio", OriginCountry: "GB"},
	}
	if err := processProductionCompanies(ctx, app.Queries, movie.ID, secondCompanies); err != nil {
		t.Fatalf("process second production companies: %v", err)
	}

	secondVideos := []tmdb.TmdbVideoResult{
		{ID: "video-1", Key: "new-key", Name: "New Featurette", Site: "Vimeo", Type: "Featurette", Official: true},
	}
	if err := processExtraVideos(ctx, app.Queries, movie.ID, secondVideos); err != nil {
		t.Fatalf("process second extra videos: %v", err)
	}

	companies, err := app.Queries.GetProductionCompaniesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get production companies: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("production companies count = %d, want 1", len(companies))
	}
	company := companies[0]
	if company.Name != "New Studio" || !company.Logo.Valid || company.Logo.String != "/new-logo.png" || !company.Country.Valid || company.Country.String != "GB" {
		t.Fatalf("production company = %+v, want refreshed mutable metadata", company)
	}

	extras, err := app.Queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extra videos: %v", err)
	}
	if len(extras) != 1 {
		t.Fatalf("extra videos count = %d, want 1", len(extras))
	}
	extra := extras[0]
	if extra.Title != "New Featurette" || extra.Key != "new-key" || extra.Type != "special_feature" || extra.Site != "vimeo" || !extra.Official {
		t.Fatalf("extra video = %+v, want refreshed mutable metadata", extra)
	}
}

func TestResolveMovieFileFallsBackWhenTmdbUnavailable(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	resolved, err := app.resolveMovieFile(context.Background(), helpers.ScanFile{
		Path: "/movies/Local.Only.2024.mkv",
		Ext:  "mkv",
		Size: 321,
	})
	if err != nil {
		t.Fatalf("resolve movie without tmdb: %v", err)
	}
	if resolved.tmdbMovie != nil {
		t.Fatal("expected no tmdb movie when TMDB is not configured")
	}
	if resolved.params.Title != "Local Only" {
		t.Fatalf("title = %q, want Local Only", resolved.params.Title)
	}
	if !resolved.params.Year.Valid || resolved.params.Year.Int64 != 2024 {
		t.Fatalf("year = %+v, want 2024", resolved.params.Year)
	}
	if resolved.params.Size != 321 {
		t.Fatalf("size = %d, want filesystem size 321", resolved.params.Size)
	}
}

func TestResolveMovieFileFallsBackWhenTmdbDetailFails(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	tmdbStub := &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{{
			TmdbID:      42,
			Title:       "Detail Fails",
			ReleaseDate: "2022-01-01",
		}},
		detailErr: sql.ErrNoRows,
	}
	app.Tmdb = tmdbStub
	app.Ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}

	resolved, err := app.resolveMovieFile(context.Background(), helpers.ScanFile{
		Path: "/movies/Detail.Fails.2022.mkv",
		Ext:  "mkv",
		Size: 123,
	})
	if err != nil {
		t.Fatalf("resolve movie with failing tmdb detail: %v", err)
	}
	if resolved.tmdbMovie != nil {
		t.Fatal("expected scanner to fall back when TMDB detail fetch fails")
	}
	if resolved.params.Title != "Detail Fails" {
		t.Fatalf("title = %q, want filename title Detail Fails", resolved.params.Title)
	}
	if !resolved.params.Year.Valid || resolved.params.Year.Int64 != 2022 {
		t.Fatalf("year = %+v, want filename year 2022", resolved.params.Year)
	}
	if len(tmdbStub.detailCalls) != 1 || tmdbStub.detailCalls[0] != 42 {
		t.Fatalf("detail calls = %#v, want [42]", tmdbStub.detailCalls)
	}
}

func TestRunMovieScanWalksVideoFilesAndLogsOnlyFinalResults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	moviesDir := t.TempDir()
	for i := 0; i < scannerBatchSize+1; i++ {
		path := filepath.Join(moviesDir, "Movie."+strconv.Itoa(i)+".2020.mkv")
		if err := os.WriteFile(path, []byte("movie"), 0o644); err != nil {
			t.Fatalf("write movie %d: %v", i, err)
		}
	}
	if err := os.WriteFile(filepath.Join(moviesDir, "not-a-movie.txt"), []byte("text"), 0o644); err != nil {
		t.Fatalf("write non-video file: %v", err)
	}

	logger := &capturedLogger{}
	app.Logger = logger
	ffprobeStub := &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
	app.Ffprobe = ffprobeStub
	app.Settings = &database.Setting{MoviesDir: sql.NullString{String: moviesDir, Valid: true}}

	app.runMovieScan()

	if ffprobeStub.calls != scannerBatchSize+1 {
		t.Fatalf("ffprobe calls = %d, want %d video files only", ffprobeStub.calls, scannerBatchSize+1)
	}

	foundCompletion := false
	wantCompletion := "movies scanner completed: " + strconv.Itoa(scannerBatchSize+1) + " scanned, 0 skipped, 0 errors"
	for _, entry := range logger.infoEntries {
		if strings.Contains(entry.msg, "movies scanner batch processed") {
			t.Fatalf("unexpected per-batch log entry: %q", entry.msg)
		}
		if strings.Contains(entry.msg, wantCompletion) {
			foundCompletion = true
		}
	}
	if !foundCompletion {
		t.Fatalf("missing final completion log; info entries = %+v", logger.infoEntries)
	}
}

func movieGenreTags(genres []database.GetGenresByMovieIDRow) string {
	tags := make([]string, 0, len(genres))
	for _, genre := range genres {
		tags = append(tags, genre.Tag)
	}
	return strings.Join(tags, ",")
}

func TestExtractYearFromReleaseDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid date format YYYY-MM-DD",
			input:    "2023-12-25",
			expected: 2023,
		},
		{
			name:     "valid date with single digit month/day",
			input:    "2023-1-5",
			expected: 2023,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "too short string",
			input:    "202",
			expected: 0,
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: 0,
		},
		{
			name:     "only year",
			input:    "2023",
			expected: 2023,
		},
		{
			name:     "year with trailing text",
			input:    "2023-12-25T00:00:00",
			expected: 2023,
		},
		{
			name:     "year 2000",
			input:    "2000-01-01",
			expected: 2000,
		},
		{
			name:     "year 1999",
			input:    "1999-12-31",
			expected: 1999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractYearFromReleaseDate(tt.input)
			if result != tt.expected {
				t.Errorf("extractYearFromReleaseDate(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetOrCreateArtist(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()

	t.Run("creates new artist", func(t *testing.T) {
		tmdbID := 12345
		name := "Test Artist"
		profilePath := "/test/profile.jpg"

		artist, err := getOrCreateArtist(ctx, app.Queries, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("getOrCreateArtist failed: %v", err)
		}

		if artist == nil {
			t.Fatal("Expected non-nil artist")
		}

		if artist.Name != name {
			t.Errorf("Expected artist name %q, got %q", name, artist.Name)
		}

		if artist.TmdbID != int64(tmdbID) {
			t.Errorf("Expected TMDB ID %d, got %d", tmdbID, artist.TmdbID)
		}
	})

	t.Run("upsert refreshes mutable metadata", func(t *testing.T) {
		tmdbID := 22222

		firstArtist, err := getOrCreateArtist(ctx, app.Queries, tmdbID, "Old Artist", "")
		if err != nil {
			t.Fatalf("first getOrCreateArtist failed: %v", err)
		}
		if firstArtist == nil {
			t.Fatal("first getOrCreateArtist returned nil artist")
		}

		secondArtist, err := getOrCreateArtist(ctx, app.Queries, tmdbID, "New Artist", "/new/profile.jpg")
		if err != nil {
			t.Fatalf("second getOrCreateArtist failed: %v", err)
		}
		if secondArtist == nil {
			t.Fatal("second getOrCreateArtist returned nil artist")
		}
		if secondArtist.ID != firstArtist.ID {
			t.Fatalf("artist ID = %d, want cached ID %d", secondArtist.ID, firstArtist.ID)
		}
		if secondArtist.Name != "New Artist" || !secondArtist.Profile.Valid || secondArtist.Profile.String != "/new/profile.jpg" {
			t.Fatalf("artist = %+v, want refreshed name/profile", secondArtist)
		}

		var name string
		var profile sql.NullString
		err = app.DB.QueryRow("SELECT name, profile FROM artist WHERE tmdb_id = ?", tmdbID).Scan(&name, &profile)
		if err != nil {
			t.Fatalf("query artist: %v", err)
		}
		if name != "New Artist" || !profile.Valid || profile.String != "/new/profile.jpg" {
			t.Fatalf("stored artist name/profile = %q/%+v, want refreshed metadata", name, profile)
		}
	})

	t.Run("empty profile path handles null", func(t *testing.T) {
		tmdbID := 11111
		name := "No Profile Artist"
		profilePath := ""

		artist, err := getOrCreateArtist(ctx, app.Queries, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("getOrCreateArtist failed: %v", err)
		}

		if artist.Profile.Valid {
			t.Error("Expected profile to be invalid for empty path")
		}
	})
}

func TestProcessMoviesBatchSharedActorIsUpsertedOncePerScan(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	moviesDir := t.TempDir()
	matrixPath := filepath.Join(moviesDir, "The.Matrix.1999.mkv")
	wickPath := filepath.Join(moviesDir, "John.Wick.2014.mkv")
	for _, p := range []string{matrixPath, wickPath} {
		if err := os.WriteFile(p, []byte("movie"), 0o644); err != nil {
			t.Fatalf("write movie: %v", err)
		}
	}

	sharedCast := `"credits": {
		"cast": [
			{"id": 6384, "name": "Keanu Reeves", "character": "Lead", "profile_path": "/keanu.jpg", "order": 0}
		],
		"crew": []
	}`
	matrixDetails := tmdbMovieFromJSON(t, `{
		"id": 603,
		"title": "The Matrix",
		"original_title": "The Matrix",
		"release_date": "1999-03-31",
		"adult": false,
		"runtime": 136,
		`+sharedCast+`
	}`)
	wickDetails := tmdbMovieFromJSON(t, `{
		"id": 245891,
		"title": "John Wick",
		"original_title": "John Wick",
		"release_date": "2014-10-24",
		"adult": false,
		"runtime": 101,
		`+sharedCast+`
	}`)

	app.Tmdb = &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 603, Title: "The Matrix", ReleaseDate: "1999-03-31"},
			{TmdbID: 245891, Title: "John Wick", ReleaseDate: "2014-10-24"},
		},
		detailMovies: map[int]tmdb.TmdbMovie{603: matrixDetails, 245891: wickDetails},
	}
	app.Ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("5432.4")}

	scanned, skipped, errCount := app.processMoviesBatch(ctx, newMovieScanContext(nil), []helpers.ScanFile{
		{Path: matrixPath, Ext: "mkv", Size: 5},
		{Path: wickPath, Ext: "mkv", Size: 6},
	})
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, app.DB, "SELECT COUNT(*) FROM artist WHERE tmdb_id = 6384"); got != 1 {
		t.Fatalf("artist rows for shared actor = %d, want 1", got)
	}

	var name string
	err := app.DB.QueryRowContext(ctx, "SELECT name FROM artist WHERE tmdb_id = 6384").Scan(&name)
	if err != nil {
		t.Fatalf("read shared artist: %v", err)
	}
	if name != "Keanu Reeves" {
		t.Fatalf("shared artist name = %q, want Keanu Reeves", name)
	}

	// Both movies' cast rows must reference the single shared artist row.
	if got := countScannerRows(t, app.DB, `
		SELECT COUNT(*)
		FROM cast AS c
		INNER JOIN artist AS a ON a.id = c.artist_id
		WHERE a.tmdb_id = 6384`); got != 2 {
		t.Fatalf("cast rows referencing shared artist = %d, want 2", got)
	}
}

func TestStreamIndexUniquePerMovie(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	result, err := app.DB.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult, duration)
		VALUES ('Unique Index Movie', '/tmp/unique-index.mkv', 'unique-index.mkv', 1, 'mkv', 'video/x-matroska', 0, 3600.0)
	`)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movieID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("movie id: %v", err)
	}

	tables := []struct {
		name   string
		insert string
	}{
		{"video_streams", `INSERT INTO video_streams (movie_id, stream_index, codec, bit_rate, width, height, frame_rate) VALUES (?, 0, 'h264', 5000000, 1920, 1080, 23.976)`},
		{"audio_streams", `INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels) VALUES (?, 1, 'aac', 192000, 2)`},
		{"subtitles", `INSERT INTO subtitles (movie_id, stream_index, codec) VALUES (?, 2, 'subrip')`},
	}
	for _, table := range tables {
		_, err = app.DB.Exec(table.insert, movieID)
		if err != nil {
			t.Fatalf("first %s insert: %v", table.name, err)
		}
		_, err = app.DB.Exec(table.insert, movieID)
		if err == nil {
			t.Fatalf("expected duplicate (movie_id, stream_index) insert into %s to fail", table.name)
		}
		if !strings.Contains(err.Error(), "UNIQUE") {
			t.Fatalf("expected UNIQUE constraint error for %s, got %v", table.name, err)
		}
	}
}
