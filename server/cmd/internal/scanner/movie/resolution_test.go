package movie

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/scanner/scannertest"
	"igloo/cmd/internal/scanner/tmdbmatch"
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

func TestResolveMovieFileReturnsTmdbSearchFailure(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	logged := &scannertest.Logger{}
	testScanner.scanner.logger = logged
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("3600")}
	testScanner.scanner.tmdb = &stubMovieScannerTmdb{searchErr: errors.New("tmdb unavailable")}

	path := "/movies/Search.Failed.2024.mkv"
	resolved, err := testScanner.scanner.resolveMovieFile(context.Background(), scanner.ScanFile{
		Path: path,
		Ext:  "mkv",
		Size: 321,
	})
	if err == nil || resolved != nil {
		t.Fatalf("provider failure was treated as a no-match: %v", err)
	}

	if !logged.WarnMentions("TMDB movie search failed", path) {
		t.Fatalf("expected a warning naming %q, got %+v", path, logged.WarnEntries)
	}
}

func TestResolveMovieFileDoesNotWarnWhenScanIsCanceled(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	logged := &scannertest.Logger{}
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

	if len(logged.WarnEntries) != 0 {
		t.Fatalf("a canceled scan should not warn about TMDB, got %+v", logged.WarnEntries)
	}
}

func TestResolveMovieFileReturnsTmdbDetailFailure(t *testing.T) {
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
	if !errors.Is(err, sql.ErrNoRows) || resolved != nil {
		t.Fatalf("detail failure was treated as a no-match: %v", err)
	}
	if len(tmdbStub.detailCalls) != 1 || tmdbStub.detailCalls[0] != 42 {
		t.Fatalf("detail calls = %#v, want [42]", tmdbStub.detailCalls)
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
			if failure != "duplicate" && !s.logger.(*scannertest.Logger).WarnMentions("TMDB movie search failed", "Blade.Runner.2049.mkv") {
				t.Fatal("operational error was suppressed")
			}
		})
	}
}

// resolvedMovieFile joins the local probe and the TMDB lookup for one file so
// resolution tests can assert on both without running the persistence phases.
type resolvedMovieFile struct {
	*localMovie
	tmdbMovie *tmdb.TmdbMovie
}

func (s *Scanner) resolveMovieFile(ctx context.Context, file scanner.ScanFile) (*resolvedMovieFile, error) {
	local, err := s.resolveLocalMovie(ctx, file, movieScanEntry{})
	if err != nil {
		return nil, err
	}
	titleYear := movieTitleYear(file.Path)
	searchTitle := tmdbmatch.NormalizeTitleForSearch(titleYear.Title)
	if searchTitle == "" {
		searchTitle = titleYear.Title
	}
	details, err := s.lookupTmdbMovie(ctx, file.Path, searchTitle, titleYear.Year, sql.NullInt64{})
	if err != nil {
		return nil, err
	}
	return &resolvedMovieFile{localMovie: local, tmdbMovie: details}, nil
}
