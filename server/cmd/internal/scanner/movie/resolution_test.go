package movie

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"

	_ "github.com/mattn/go-sqlite3"
)

func TestResolveMovieFilePinsMimeTypePerContainer(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}

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
		resolved, err := testScanner.scanner.resolveMovieFile(context.Background(), scanner.ScanFile{
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

// bestTmdbMatch is the top-ranked candidate, or nil when there are none. The
// scanner uses the same ranking as the manual picker.
func bestTmdbMatch(results []tmdb.TmdbMovie, targetTitle string, targetYear int) *TMDBMovieMatch {
	ranked := RankTMDBMovies(results, targetTitle, targetYear)
	if len(ranked) == 0 {
		return nil
	}
	return ranked[0]
}

func TestSelectBestTmdbMatch(t *testing.T) {
	t.Run("empty results returns nil", func(t *testing.T) {
		results := []tmdb.TmdbMovie{}
		result := bestTmdbMatch(results, "test movie", 2023)
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
		result := bestTmdbMatch(results, "Test Movie", 2023)
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
		result := bestTmdbMatch(results, "Moneyball", 2011)
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
		result := bestTmdbMatch(results, "Train Dreams", 2025)
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
		result := bestTmdbMatch(results, "Goldfinger", 1964)
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

	ranked := RankTMDBMovies(results, "Casino Royale", 2006)
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
		{
			input: "The.Hobbit.An.Unexpected.Journey.2012.1080p.x265.10bit",
			want:  "the hobbit an unexpected journey 2012",
		},
		{
			input: "Rabbit.Hole.2010.720p.8bit",
			want:  "rabbit hole 2010",
		},
		{
			input: "Orbit.2022.10bit",
			want:  "orbit 2022",
		},
		{
			input: "Gambit.2012.8bit",
			want:  "gambit 2012",
		},
	}

	for _, tt := range tests {
		got := NormalizeTitleForSearch(tt.input)
		if got != tt.want {
			t.Errorf("normalizeMovieTitleForSearch(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeComparableMovieTitlePreservesWordsEndingInBit(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "The Hobbit: An Unexpected Journey", want: "the hobbit an unexpected journey"},
		{input: "Rabbit Hole", want: "rabbit hole"},
		{input: "Orbit!", want: "orbit"},
		{input: "Gambit (2012)", want: "gambit 2012"},
	}

	for _, tt := range tests {
		got := normalizeComparableMovieTitle(tt.input)
		if got != tt.want {
			t.Errorf("normalizeComparableMovieTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveMovieFileFallsBackWhenTmdbUnavailable(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	resolved, err := testScanner.scanner.resolveMovieFile(context.Background(), scanner.ScanFile{
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

func TestResolveMovieFileLogsTmdbSearchFailure(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	logged := &capturedLogger{}
	testScanner.scanner.logger = logged
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	testScanner.scanner.tmdb = &stubMovieScannerTmdb{searchErr: errors.New("tmdb unavailable")}

	path := "/movies/Search.Failed.2024.mkv"
	resolved, err := testScanner.scanner.resolveMovieFile(context.Background(), scanner.ScanFile{
		Path: path,
		Ext:  "mkv",
		Size: 321,
	})
	if err != nil {
		t.Fatalf("resolve movie with failing tmdb search: %v", err)
	}
	if resolved.tmdbMovie != nil {
		t.Fatal("expected no tmdb movie when the search fails")
	}

	if !warnEntryMentions(logged, "TMDB movie search failed", path) {
		t.Fatalf("expected a warning naming %q, got %+v", path, logged.warnEntries)
	}
}

func TestResolveMovieFileDoesNotWarnWhenScanIsCanceled(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	logged := &capturedLogger{}
	testScanner.scanner.logger = logged
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	testScanner.scanner.tmdb = &stubMovieScannerTmdb{searchErr: context.Canceled}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := testScanner.scanner.resolveMovieFile(ctx, scanner.ScanFile{
		Path: "/movies/Canceled.2024.mkv",
		Ext:  "mkv",
		Size: 321,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resolve canceled movie: %v", err)
	}

	logged.mu.Lock()
	warnings := len(logged.warnEntries)
	logged.mu.Unlock()
	if warnings != 0 {
		t.Fatalf("a canceled scan should not warn about TMDB, got %+v", logged.warnEntries)
	}
}

// warnEntryMentions reports whether a warning with the given message carries
// needle in any of its structured values.
func warnEntryMentions(logged *capturedLogger, msg, needle string) bool {
	logged.mu.Lock()
	defer logged.mu.Unlock()

	for _, entry := range logged.warnEntries {
		if entry.msg != msg {
			continue
		}
		for _, arg := range entry.args {
			value, ok := arg.(string)
			if ok && strings.Contains(value, needle) {
				return true
			}
		}
	}

	return false
}

func TestResolveMovieFileFallsBackWhenTmdbDetailFails(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	tmdbStub := &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{{
			TmdbID:      42,
			Title:       "Detail Fails",
			ReleaseDate: "2022-01-01",
		}},
		detailErr: sql.ErrNoRows,
	}
	testScanner.scanner.tmdb = tmdbStub
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}

	resolved, err := testScanner.scanner.resolveMovieFile(context.Background(), scanner.ScanFile{
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

func TestTMDBRankingTitleDominatesYearAndPopularity(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		year        int
		candidates  []tmdb.TmdbMovie
		want        int
	}{
		{"unrelated same year", "Arrival", 2016, []tmdb.TmdbMovie{{TmdbID: 1, Title: "Arrival", ReleaseDate: "2015-01-01"}, {TmdbID: 2, Title: "Random Film", ReleaseDate: "2016-01-01", Popularity: 1e9, VoteAverage: 1e9}}, 1},
		{"adjacent release", "Arrival", 2016, []tmdb.TmdbMovie{{TmdbID: 1, Title: "Arrival", ReleaseDate: "2015-01-01"}, {TmdbID: 2, Title: "Arrival", ReleaseDate: "2012-01-01", Popularity: 1e9, VoteAverage: 10}}, 1},
		{"sequel", "Rocky II", 1979, []tmdb.TmdbMovie{{TmdbID: 1, Title: "Rocky II", ReleaseDate: "1979-01-01"}, {TmdbID: 2, Title: "Rocky", ReleaseDate: "1979-01-01", Popularity: 1e9, VoteAverage: 10}}, 1},
		{"deterministic ties", "Movie", 0, []tmdb.TmdbMovie{{TmdbID: 2, Title: "Movie"}, {TmdbID: 1, Title: "Movie"}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ranked := RankTMDBMovies(tc.candidates, tc.title, tc.year)
			if ranked[0].Movie.TmdbID != tc.want {
				t.Fatalf("winner=%+v", ranked[0])
			}
		})
	}
}

type interpretationTmdb struct {
	stubMovieScannerTmdb
	search func(context.Context, string, int) ([]tmdb.TmdbMovie, error)
}

func (s *interpretationTmdb) SearchMoviesByTitleAndYear(ctx context.Context, title string, years ...int) ([]tmdb.TmdbMovie, error) {
	year := 0
	if len(years) > 0 {
		year = years[0]
	}
	s.searchCalls = append(s.searchCalls, stubMovieScannerTmdbSearchCall{title: title, year: years})
	return s.search(ctx, title, year)
}

func TestAmbiguousTitleYearSearches(t *testing.T) {
	for _, tc := range []struct {
		filename, parsed, full string
		year, calls            int
	}{
		{"Blade.Runner.2049.mkv", "blade runner", "blade runner 2049", 2049, 2},
		{"Wonder.Woman.1984.mkv", "wonder woman", "wonder woman 1984", 1984, 2},
		{"Blade.Runner.2049.2017.mkv", "blade runner 2049", "blade runner 2049", 2017, 1},
		{"Wonder Woman 1984 (2020).mkv", "wonder woman 1984", "wonder woman 1984", 2020, 1},
		{"Blade Runner (1982).mkv", "blade runner", "blade runner", 1982, 1},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
			s := fixture.scanner
			s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			correct := tmdb.TmdbMovie{TmdbID: 2, Title: tc.full, OriginalTitle: tc.full, ReleaseDate: "2017-01-01"}
			wrong := tmdb.TmdbMovie{TmdbID: 1, Title: tc.parsed, OriginalTitle: tc.parsed, ReleaseDate: "1980-01-01"}
			client := &interpretationTmdb{stubMovieScannerTmdb: stubMovieScannerTmdb{detailMovies: map[int]tmdb.TmdbMovie{2: correct}}}
			client.search = func(_ context.Context, title string, year int) ([]tmdb.TmdbMovie, error) {
				if len(client.searchCalls) == 1 && (title != tc.parsed || year != tc.year) {
					t.Fatalf("parsed query=%s/%d", title, year)
				}
				if len(client.searchCalls) == 2 && (title != tc.full || year != 0) {
					t.Fatalf("full query=%s/%d", title, year)
				}
				if tc.calls == 1 {
					return []tmdb.TmdbMovie{correct}, nil
				}
				return []tmdb.TmdbMovie{wrong, correct}, nil
			}
			s.tmdb = client
			resolved, err := s.resolveMovieFile(context.Background(), scanner.ScanFile{Path: tc.filename, Ext: "mkv"})
			if err != nil {
				t.Fatal(err)
			}
			if resolved.tmdbMovie == nil || resolved.tmdbMovie.TmdbID != 2 || len(client.searchCalls) != tc.calls || len(client.detailCalls) != 1 {
				t.Fatalf("match=%+v searches=%+v details=%+v", resolved.tmdbMovie, client.searchCalls, client.detailCalls)
			}
		})
	}
}

func TestAmbiguousSearchPartialFailuresAndCancellation(t *testing.T) {
	for _, failure := range []string{"first", "second", "cancel", "duplicate"} {
		t.Run(failure, func(t *testing.T) {
			fixture := setupMovieScanner(t)
			defer fixture.db.Close()
			s := fixture.scanner
			s.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("120")}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			details := tmdb.TmdbMovie{TmdbID: 2, Title: "Blade Runner 2049"}
			client := &interpretationTmdb{stubMovieScannerTmdb: stubMovieScannerTmdb{detailMovies: map[int]tmdb.TmdbMovie{2: details}}}
			client.search = func(_ context.Context, _ string, _ int) ([]tmdb.TmdbMovie, error) {
				call := len(client.searchCalls)
				if failure == "cancel" {
					cancel()
					return nil, context.Canceled
				}
				if failure == "first" && call == 1 || failure == "second" && call == 2 {
					return nil, errors.New("search unavailable")
				}
				if failure == "duplicate" && call == 2 {
					return []tmdb.TmdbMovie{{TmdbID: 2, Title: "Weak duplicate"}, {TmdbID: 3, Title: "Blade"}}, nil
				}
				return []tmdb.TmdbMovie{details}, nil
			}
			s.tmdb = client
			resolved, err := s.resolveMovieFile(ctx, scanner.ScanFile{Path: "Blade.Runner.2049.mkv", Ext: "mkv"})
			if err != nil && failure != "cancel" {
				t.Fatal(err)
			}
			if failure == "cancel" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation error=%v", err)
				}
				if len(client.searchCalls) != 1 || len(client.detailCalls) != 0 {
					t.Fatal("cancellation did not stop search")
				}
				return
			}
			if resolved.tmdbMovie == nil || resolved.tmdbMovie.TmdbID != 2 || len(client.detailCalls) != 1 {
				t.Fatalf("partial search discarded candidate: %+v", resolved)
			}
			if failure != "duplicate" && !warnEntryMentions(s.logger.(*capturedLogger), "TMDB movie search failed", "Blade.Runner.2049.mkv") {
				t.Fatal("operational error was suppressed")
			}
		})
	}
}

func (s *Scanner) resolveMovieFile(ctx context.Context, file scanner.ScanFile) (*resolvedMovie, error) {
	return s.resolveMovie(ctx, file, true, false)
}
