package movie

import (
	"context"
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/ffprobe"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner"
	"igloo/cmd/internal/tmdb"

	_ "github.com/mattn/go-sqlite3"
)

func TestPersistResolvedMovieInvalidatesAfterCommit(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	var invalidatedID int64
	callbackObservedCommittedRow := false
	testScanner.scanner.invalidateCommittedMovie = func(movieID int64) {
		invalidatedID = movieID
		movie, err := testScanner.queries.GetMovieByID(context.Background(), movieID)
		callbackObservedCommittedRow = err == nil && movie.ID == movieID
	}

	resolved := &resolvedMovie{params: database.UpsertMovieParams{
		Title:     "Committed Movie",
		FilePath:  "/movies/committed.mkv",
		FileName:  "committed.mkv",
		Size:      1,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
	}}
	err := testScanner.scanner.persistResolvedMovie(context.Background(), newMovieScanContext(nil), resolved)
	if err == nil {
		t.Fatal("persist without a video stream unexpectedly succeeded")
	}
	if invalidatedID != 0 || callbackObservedCommittedRow {
		t.Fatal("invalidation ran for a rolled-back movie")
	}

	resolved.streams = movieScannerMetadataFixture("120").Streams
	err = testScanner.scanner.persistResolvedMovie(context.Background(), newMovieScanContext(nil), resolved)
	if err != nil {
		t.Fatalf("persist resolved movie: %v", err)
	}
	if invalidatedID == 0 || !callbackObservedCommittedRow {
		t.Fatal("invalidation did not observe the committed movie")
	}
}

func TestMovieScannerUpsertPreservesAudienceRatingAndRefreshesMetadata(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	ctx := context.Background()
	path := "/movies/Moneyball.2011.mkv"

	_, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
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

	updated, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
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

func TestProcessMoviesBatchWithTmdbPersistsMetadataRelationshipsAndStreams(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

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
	testScanner.scanner.tmdb = tmdbStub
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("5432.4")}

	scanned, skipped, errCount := testScanner.scanner.processMoviesBatch(ctx, newMovieScanContext(nil), []scanner.ScanFile{
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
	err := testScanner.db.QueryRowContext(ctx, `
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

	genres, err := testScanner.queries.GetGenresByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	if got := movieGenreTags(genres); got != "Action,Science Fiction" {
		t.Fatalf("genres = %q, want Action,Science Fiction", got)
	}

	cast, err := testScanner.queries.GetCastByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get cast: %v", err)
	}
	if len(cast) != 1 || cast[0].ArtistName != "Keanu Reeves" || cast[0].Character != "Neo" {
		t.Fatalf("cast = %+v, want Keanu Reeves as Neo", cast)
	}

	crew, err := testScanner.queries.GetCrewByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get crew: %v", err)
	}
	if len(crew) != 1 || crew[0].ArtistName != "Lana Wachowski" || crew[0].Job != "Director" {
		t.Fatalf("crew = %+v, want Lana Wachowski Director", crew)
	}

	companies, err := testScanner.queries.GetProductionCompaniesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get production companies: %v", err)
	}
	if len(companies) != 1 || companies[0].Name != "Warner Bros." || companies[0].TmdbID != 174 {
		t.Fatalf("production companies = %+v, want Warner Bros.", companies)
	}

	extras, err := testScanner.queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extra videos: %v", err)
	}
	if len(extras) != 1 || extras[0].Title != "Official Trailer" || extras[0].Type != "trailer" || extras[0].Site != "youtube" {
		t.Fatalf("extra videos = %+v, want mapped YouTube trailer", extras)
	}

	videoStreams, err := testScanner.queries.GetVideoStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get video streams: %v", err)
	}
	if len(videoStreams) != 1 || videoStreams[0].StreamIndex != 0 || videoStreams[0].Codec != "h264" {
		t.Fatalf("video streams = %+v, want one h264 non-cover stream", videoStreams)
	}

	audioStreams, err := testScanner.queries.GetAudioStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get audio streams: %v", err)
	}
	if len(audioStreams) != 1 || audioStreams[0].StreamIndex != 2 || audioStreams[0].Channels != 6 {
		t.Fatalf("audio streams = %+v, want one 5.1 audio stream", audioStreams)
	}

	subtitles, err := testScanner.queries.GetSubtitlesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get subtitles: %v", err)
	}
	if len(subtitles) != 1 || subtitles[0].StreamIndex != 3 || subtitles[0].Codec != "subrip" {
		t.Fatalf("subtitles = %+v, want one subrip subtitle", subtitles)
	}

	chapters, err := testScanner.queries.GetChaptersByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get chapters: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Opening" || chapters[1].StartTime != 120 {
		t.Fatalf("chapters = %+v, want two normalized chapters", chapters)
	}
}

func TestProcessMoviesBatchWithTmdbReplacesScannerOwnedRelationshipsOnRescan(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

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
	testScanner.scanner.tmdb = tmdbStub
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{
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
	scanned, skipped, errCount := testScanner.scanner.processMoviesBatch(ctx, scan, []scanner.ScanFile{
		{Path: path, Ext: "mkv", Size: 5},
	})
	if scanned != 1 || skipped != 0 || errCount != 0 {
		t.Fatalf("first scan result scanned=%d skipped=%d errors=%d, want 1/0/0", scanned, skipped, errCount)
	}

	tmdbStub.detailMovies[1000] = secondDetails
	scanned, skipped, errCount = testScanner.scanner.processMoviesBatch(ctx, scan, []scanner.ScanFile{
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
	err := testScanner.db.QueryRowContext(ctx, `
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

	genres, err := testScanner.queries.GetGenresByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get genres: %v", err)
	}
	if got := movieGenreTags(genres); got != "Drama" {
		t.Fatalf("genres after rescan = %q, want Drama", got)
	}

	cast, err := testScanner.queries.GetCastByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get cast: %v", err)
	}
	if len(cast) != 1 || cast[0].ArtistName != "Second Actor" || cast[0].Character != "New Role" {
		t.Fatalf("cast after rescan = %+v, want only second actor", cast)
	}

	crew, err := testScanner.queries.GetCrewByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get crew: %v", err)
	}
	if len(crew) != 1 || crew[0].ArtistName != "Second Director" {
		t.Fatalf("crew after rescan = %+v, want only second director", crew)
	}

	companies, err := testScanner.queries.GetProductionCompaniesByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get production companies: %v", err)
	}
	if len(companies) != 1 || companies[0].Name != "New Studio" {
		t.Fatalf("production companies after rescan = %+v, want New Studio", companies)
	}

	extras, err := testScanner.queries.GetMovieExtraVideos(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get extra videos: %v", err)
	}
	if len(extras) != 1 || extras[0].Title != "New Trailer" || extras[0].Type != "special_feature" || extras[0].Site != "vimeo" {
		t.Fatalf("extra videos after rescan = %+v, want mapped new featurette", extras)
	}

	videoStreams, err := testScanner.queries.GetVideoStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get video streams: %v", err)
	}
	if len(videoStreams) != 1 || videoStreams[0].StreamIndex != 4 || videoStreams[0].Codec != "hevc" {
		t.Fatalf("video streams after rescan = %+v, want one new hevc stream", videoStreams)
	}

	audioStreams, err := testScanner.queries.GetAudioStreamsByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get audio streams: %v", err)
	}
	if len(audioStreams) != 1 || audioStreams[0].StreamIndex != 5 {
		t.Fatalf("audio streams after rescan = %+v, want one new audio stream", audioStreams)
	}

	chapters, err := testScanner.queries.GetChaptersByMovieID(ctx, movie.ID)
	if err != nil {
		t.Fatalf("get chapters: %v", err)
	}
	if len(chapters) != 1 || chapters[0].Title != "Only New Chapter" || chapters[0].StartTime != 30 {
		t.Fatalf("chapters after rescan = %+v, want one new chapter", chapters)
	}
}

func TestMovieScannerEntityUpsertRefreshesMutableMetadata(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	ctx := context.Background()
	movie, err := testScanner.queries.UpsertMovie(ctx, database.UpsertMovieParams{
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

	firstCompanies := []tmdb.ProductionCompany{
		{ID: 100, LogoPath: "/old-logo.png", Name: "Old Studio", OriginCountry: "US"},
	}
	if err := processProductionCompanies(ctx, testScanner.queries, movie.ID, firstCompanies); err != nil {
		t.Fatalf("process first production companies: %v", err)
	}

	firstVideos := []tmdb.TmdbVideoResult{
		{ID: "video-1", Key: "old-key", Name: "Old Trailer", Site: "YouTube", Type: "Trailer", Official: false},
	}
	if err := processExtraVideos(ctx, testScanner.queries, movie.ID, firstVideos); err != nil {
		t.Fatalf("process first extra videos: %v", err)
	}

	secondCompanies := []tmdb.ProductionCompany{
		{ID: 100, LogoPath: "/new-logo.png", Name: "New Studio", OriginCountry: "GB"},
	}
	if err := processProductionCompanies(ctx, testScanner.queries, movie.ID, secondCompanies); err != nil {
		t.Fatalf("process second production companies: %v", err)
	}

	secondVideos := []tmdb.TmdbVideoResult{
		{ID: "video-1", Key: "new-key", Name: "New Featurette", Site: "Vimeo", Type: "Featurette", Official: true},
	}
	if err := processExtraVideos(ctx, testScanner.queries, movie.ID, secondVideos); err != nil {
		t.Fatalf("process second extra videos: %v", err)
	}

	companies, err := testScanner.queries.GetProductionCompaniesByMovieID(ctx, movie.ID)
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

	extras, err := testScanner.queries.GetMovieExtraVideos(ctx, movie.ID)
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

func movieGenreTags(genres []database.GetGenresByMovieIDRow) string {
	tags := make([]string, 0, len(genres))
	for _, genre := range genres {
		tags = append(tags, genre.Tag)
	}
	return strings.Join(tags, ",")
}

func TestGetOrCreateArtist(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

	ctx := context.Background()

	t.Run("creates new artist", func(t *testing.T) {
		tmdbID := 12345
		name := "Test Artist"
		profilePath := "/test/profile.jpg"

		artist, err := getOrCreateArtist(ctx, testScanner.queries, tmdbID, name, profilePath)
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

		firstArtist, err := getOrCreateArtist(ctx, testScanner.queries, tmdbID, "Old Artist", "")
		if err != nil {
			t.Fatalf("first getOrCreateArtist failed: %v", err)
		}
		if firstArtist == nil {
			t.Fatal("first getOrCreateArtist returned nil artist")
		}

		secondArtist, err := getOrCreateArtist(ctx, testScanner.queries, tmdbID, "New Artist", "/new/profile.jpg")
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
		err = testScanner.db.QueryRow("SELECT name, profile FROM artist WHERE tmdb_id = ?", tmdbID).Scan(&name, &profile)
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

		artist, err := getOrCreateArtist(ctx, testScanner.queries, tmdbID, name, profilePath)
		if err != nil {
			t.Fatalf("getOrCreateArtist failed: %v", err)
		}

		if artist.Profile.Valid {
			t.Error("Expected profile to be invalid for empty path")
		}
	})
}

func TestProcessMoviesBatchSharedActorIsUpsertedOncePerScan(t *testing.T) {
	testScanner := setupMovieScanner(t)
	defer testScanner.db.Close()

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

	testScanner.scanner.tmdb = &stubMovieScannerTmdb{
		searchResults: []tmdb.TmdbMovie{
			{TmdbID: 603, Title: "The Matrix", ReleaseDate: "1999-03-31"},
			{TmdbID: 245891, Title: "John Wick", ReleaseDate: "2014-10-24"},
		},
		detailMovies: map[int]tmdb.TmdbMovie{603: matrixDetails, 245891: wickDetails},
	}
	testScanner.scanner.ffprobe = &stubMovieScannerFfprobe{result: movieScannerMetadataFixture("5432.4")}

	scanned, skipped, errCount := testScanner.scanner.processMoviesBatch(ctx, newMovieScanContext(nil), []scanner.ScanFile{
		{Path: matrixPath, Ext: "mkv", Size: 5},
		{Path: wickPath, Ext: "mkv", Size: 6},
	})
	if scanned != 2 || skipped != 0 || errCount != 0 {
		t.Fatalf("scan result scanned=%d skipped=%d errors=%d, want 2/0/0", scanned, skipped, errCount)
	}

	if got := countScannerRows(t, testScanner.db, "SELECT COUNT(*) FROM artist WHERE tmdb_id = 6384"); got != 1 {
		t.Fatalf("artist rows for shared actor = %d, want 1", got)
	}

	var name string
	err := testScanner.db.QueryRowContext(ctx, "SELECT name FROM artist WHERE tmdb_id = 6384").Scan(&name)
	if err != nil {
		t.Fatalf("read shared artist: %v", err)
	}
	if name != "Keanu Reeves" {
		t.Fatalf("shared artist name = %q, want Keanu Reeves", name)
	}

	// Both movies' cast rows must reference the single shared artist row.
	if got := countScannerRows(t, testScanner.db, `
		SELECT COUNT(*)
		FROM cast AS c
		INNER JOIN artist AS a ON a.id = c.artist_id
		WHERE a.tmdb_id = 6384`); got != 2 {
		t.Fatalf("cast rows referencing shared artist = %d, want 2", got)
	}
}
