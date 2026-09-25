package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"igloo/cmd/internal/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type searchAllHTTPResponse struct {
	Error   bool          `json:"error"`
	Message string        `json:"message,omitempty"`
	Data    searchAllData `json:"data"`
}

type searchMoviesHTTPResponse struct {
	Error   bool                                                `json:"error"`
	Message string                                              `json:"message,omitempty"`
	Data    searchCategoryData[database.GetMoviesLibraryAscRow] `json:"data"`
}

// searchEntityResults runs the full staged match resolution plus page query
// for one entity, mirroring what the HTTP handlers do.
func searchEntityResults[T any](t *testing.T, app *Application, e searchEntity[T], query string) []T {
	t.Helper()

	match, _, ok, err := app.resolveSearchMatch(context.Background(), e.countSQL, e.vocabTable, query)
	if err != nil {
		t.Fatalf("resolveSearchMatch(%q) failed: %v", query, err)
	}
	if !ok {
		return []T{}
	}

	results, err := searchEntityPage(context.Background(), app, e, query, match, 10, 0)
	if err != nil {
		t.Fatalf("searchEntityPage(%q) failed: %v", query, err)
	}
	return results
}

func TestSearchMoviesStagedMatching(t *testing.T) {
	app := setupTestApp(t)

	createTestMovie(t, app, "Casino Nights", "/movies/casino-nights.mkv")
	createTestMovie(t, app, "Royale Tenenbaums", "/movies/royale-tenenbaums.mkv")
	createTestMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	// A well-spelled multi-token query resolves at stage 1 (AND) and only
	// returns documents containing every token.
	results := searchEntityResults(t, app, movieSearchEntity, "Casino Royale")
	if len(results) != 1 {
		t.Fatalf("expected AND matching to return 1 movie, got %d", len(results))
	}
	if results[0].Title != "Casino Royale" {
		t.Fatalf("expected Casino Royale, got %q", results[0].Title)
	}

	// Tokens that never co-occur and have no near-spelled vocabulary terms
	// fall through to the stage-3 OR query, keeping broad recall.
	results = searchEntityResults(t, app, movieSearchEntity, "Casino Zzzqx")
	if len(results) != 2 {
		t.Fatalf("expected OR fallback to return 2 casino movies, got %d", len(results))
	}
}

func TestSearchMoviesSingleTokenTypoReturnsResult(t *testing.T) {
	app := setupTestApp(t)

	createTestMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")

	for _, query := range []string{"Lisence", "Lisense"} {
		t.Run(query, func(t *testing.T) {
			results := searchEntityResults(t, app, movieSearchEntity, query)
			if len(results) != 1 {
				t.Fatalf("expected 1 result for %q, got %d", query, len(results))
			}
			if results[0].Title != "Licence to Kill" {
				t.Fatalf("expected Licence to Kill, got %q", results[0].Title)
			}
		})
	}
}

func TestSearchShowsStagedMatching(t *testing.T) {
	app := setupTestApp(t)

	createSearchShow(t, app, "Breaking Bad", "/shows/Breaking Bad", "", "")
	createSearchShow(t, app, "Breaking Point", "/shows/Breaking Point", "", "")
	createSearchShow(t, app, "Better Call Saul", "/shows/Better Call Saul", "", "")

	// Stage 1 (AND) keeps a well-spelled multi-token query narrow.
	results := searchEntityResults(t, app, showSearchEntity, "Breaking Bad")
	if len(results) != 1 {
		t.Fatalf("expected AND matching to return 1 show, got %d", len(results))
	}
	if results[0].Name != "Breaking Bad" {
		t.Fatalf("expected Breaking Bad, got %q", results[0].Name)
	}

	// A token with no co-occurrence and no near-spelled vocabulary term falls
	// through to the stage-3 OR query.
	results = searchEntityResults(t, app, showSearchEntity, "Breaking Zzzqx")
	if len(results) != 2 {
		t.Fatalf("expected OR fallback to return 2 Breaking shows, got %d", len(results))
	}
}

func TestSearchShowsMatchesOverviewAndTagline(t *testing.T) {
	app := setupTestApp(t)

	createSearchShow(
		t, app,
		"Severance",
		"/shows/Severance (2024)",
		"Lumon employees split their memories between work and home.",
		"The work is mysterious and important.",
	)

	// The indexed text arrives through UpdateShowMetadata, so these two also
	// prove the shows_au trigger reindexes on a metadata write.
	for _, query := range []string{"Lumon", "mysterious"} {
		t.Run(query, func(t *testing.T) {
			results := searchEntityResults(t, app, showSearchEntity, query)
			if len(results) != 1 {
				t.Fatalf("expected 1 show for %q, got %d", query, len(results))
			}
			if results[0].Name != "Severance" {
				t.Fatalf("expected Severance, got %q", results[0].Name)
			}
		})
	}
}

func TestSearchShowsTypoInOneTokenRanksTargetFirst(t *testing.T) {
	app := setupTestApp(t)

	createSearchShow(t, app, "Severance", "/shows/Severance", "", "")
	createSearchShow(t, app, "Deliverance Bay", "/shows/Deliverance Bay", "", "")

	results := searchEntityResults(t, app, showSearchEntity, "Severence")
	if len(results) == 0 {
		t.Fatal("expected typo-corrected show search to return results")
	}
	if results[0].Name != "Severance" {
		t.Fatalf("expected Severance first, got %q", results[0].Name)
	}
}

// The show index ships after the shows table, so schema.sql backfills it once
// for libraries scanned before it existed.
func TestShowSearchIndexBackfillsExistingLibrary(t *testing.T) {
	app := setupTestApp(t)

	// Simulate a database whose shows predate the index.
	_, err := app.DB.Exec("DROP TRIGGER shows_ai; DROP TRIGGER shows_au")
	if err != nil {
		t.Fatalf("drop show index triggers: %v", err)
	}
	createSearchShow(t, app, "Legacy Show", "/shows/Legacy Show", "", "")

	var terms int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM shows_fts_vocab").Scan(&terms)
	if err != nil {
		t.Fatalf("count vocabulary terms: %v", err)
	}
	if terms != 0 {
		t.Fatalf("expected an empty index before the backfill, got %d terms", terms)
	}

	// Reapplying the schema is exactly what the next startup does; it restores
	// the triggers and runs the guarded rebuild.
	err = app.InitTables()
	if err != nil {
		t.Fatalf("reapply schema: %v", err)
	}

	results := searchEntityResults(t, app, showSearchEntity, "Legacy")
	if len(results) != 1 {
		t.Fatalf("expected the backfill to make 1 show searchable, got %d", len(results))
	}

	// The guard is monotonic: a second startup must not touch the index.
	err = app.DB.QueryRow("SELECT COUNT(*) FROM shows_fts_vocab").Scan(&terms)
	if err != nil {
		t.Fatalf("count vocabulary terms: %v", err)
	}
	err = app.InitTables()
	if err != nil {
		t.Fatalf("reapply schema again: %v", err)
	}
	var termsAfter int
	err = app.DB.QueryRow("SELECT COUNT(*) FROM shows_fts_vocab").Scan(&termsAfter)
	if err != nil {
		t.Fatalf("count vocabulary terms: %v", err)
	}
	if termsAfter != terms {
		t.Fatalf("second startup changed the index: %d terms, want %d", termsAfter, terms)
	}
}

func TestSearchTracksMusicianTypoReturnsResult(t *testing.T) {
	app := setupTestApp(t)

	musicianID := createSearchMusician(t, app, "Adele")
	albumID := createSearchAlbum(t, app, "Twenty Five", "Adele")
	createSearchTrack(t, app, "Hello", "/music/hello.flac", albumID, musicianID)

	results := searchEntityResults(t, app, trackSearchEntity, "Adelle")
	if len(results) != 1 {
		t.Fatalf("expected 1 track for misspelled musician, got %d", len(results))
	}
	if results[0].Title != "Hello" {
		t.Fatalf("expected Hello, got %q", results[0].Title)
	}
}

func TestSearchMoviesFTSSyntaxInputDoesNotSuppressResults(t *testing.T) {
	app := setupTestApp(t)

	createTestMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	results := searchEntityResults(t, app, movieSearchEntity, `"Casino" OR title:royale`)
	if len(results) == 0 || results[0].Title != "Casino Royale" {
		t.Fatalf("expected Casino Royale result, got %#v", results)
	}
}

func TestSearchTracksMatchesTrackAlbumAndArtist(t *testing.T) {
	app := setupTestApp(t)

	musicianID := createSearchMusician(t, app, "Adele")
	albumID := createSearchAlbum(t, app, "Twenty Five", "Adele")
	createSearchTrack(t, app, "Hello", "/music/hello.flac", albumID, musicianID)

	for _, query := range []string{"Hello", "Twenty", "Adele"} {
		t.Run(query, func(t *testing.T) {
			results := searchEntityResults(t, app, trackSearchEntity, query)
			if len(results) != 1 {
				t.Fatalf("expected 1 track for %q, got %d", query, len(results))
			}
			if results[0].Title != "Hello" {
				t.Fatalf("expected Hello, got %q", results[0].Title)
			}
		})
	}
}

func TestSearchTracksReflectsTrackRelationshipUpdates(t *testing.T) {
	app := setupTestApp(t)

	originalMusicianID := createSearchMusician(t, app, "Adele")
	updatedMusicianID := createSearchMusician(t, app, "Sia")
	albumID := createSearchAlbum(t, app, "Power Ballads", "Various Artists")
	createSearchTrack(t, app, "Hello", "/music/hello.flac", albumID, originalMusicianID)

	results := searchEntityResults(t, app, trackSearchEntity, "Sia")
	if len(results) != 0 {
		t.Fatalf("expected no Sia results before update, got %#v", results)
	}

	createSearchTrack(t, app, "Hello", "/music/hello.flac", albumID, updatedMusicianID)

	results = searchEntityResults(t, app, trackSearchEntity, "Sia")
	if len(results) != 1 || results[0].Title != "Hello" {
		t.Fatalf("expected updated track relationship to be searchable, got %#v", results)
	}

	results = searchEntityResults(t, app, trackSearchEntity, "Adele")
	if len(results) != 0 {
		t.Fatalf("expected old musician relationship to be removed from search, got %#v", results)
	}
}

func TestSearchAllRouteReturnsSameResultsForSlashVariants(t *testing.T) {
	app := setupTestApp(t)

	userID := createTestUser(t, app, "Search User", "search@example.com", false).ID
	createTestMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	app.InitSession()
	app.InitRouter()

	var previous *searchAllData
	for _, path := range []string{"/api/search?q=casino", "/api/search/?q=casino"} {
		t.Run(path, func(t *testing.T) {
			w := performAuthenticatedSearchRequest(t, app, userID, path)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", w.Code, w.Body.String())
			}

			var resp searchAllHTTPResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Error {
				t.Fatalf("expected success response, got %q", resp.Message)
			}
			if resp.Data.Query != "casino" {
				t.Fatalf("query = %q, want casino", resp.Data.Query)
			}
			if resp.Data.Movies.Total != 1 {
				t.Fatalf("movie total = %d, want 1", resp.Data.Movies.Total)
			}
			if len(resp.Data.Movies.Results) != 1 || resp.Data.Movies.Results[0].Title != "Casino Royale" {
				t.Fatalf("unexpected movie results: %#v", resp.Data.Movies.Results)
			}

			if previous != nil && resp.Data.Movies.Total != previous.Movies.Total {
				t.Fatalf("slash variant total = %d, previous total = %d", resp.Data.Movies.Total, previous.Movies.Total)
			}
			previous = &resp.Data
		})
	}
}

func TestSearchMoviesRouteCorrectsTypos(t *testing.T) {
	app := setupTestApp(t)

	userID := createTestUser(t, app, "Search User", "search@example.com", false).ID
	createTestMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")
	createTestMovie(t, app, "Kill Bill: Volume 1", "/movies/kill-bill-1.mkv")

	app.InitSession()
	app.InitRouter()

	w := performAuthenticatedSearchRequest(t, app, userID, "/api/search/movies?q=License+to+Kill")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", w.Code, w.Body.String())
	}

	var resp searchMoviesHTTPResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error {
		t.Fatalf("expected success response, got %q", resp.Message)
	}
	if len(resp.Data.Results) == 0 {
		t.Fatal("expected typo-corrected route search to return results")
	}
	if resp.Data.Results[0].Title != "Licence to Kill" {
		t.Fatalf("expected Licence to Kill first, got %q", resp.Data.Results[0].Title)
	}
}

func TestSearchMoviesRouteNormalizesPagination(t *testing.T) {
	app := setupTestApp(t)

	userID := createTestUser(t, app, "Search User", "search@example.com", false).ID
	createTestMovie(t, app, "Pageable Movie One", "/movies/pageable-1.mkv")
	createTestMovie(t, app, "Pageable Movie Two", "/movies/pageable-2.mkv")
	createTestMovie(t, app, "Pageable Movie Three", "/movies/pageable-3.mkv")

	app.InitSession()
	app.InitRouter()

	w := performAuthenticatedSearchRequest(t, app, userID, "/api/search/movies?q=Pageable&page=999&per_page=2")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", w.Code, w.Body.String())
	}

	var resp searchMoviesHTTPResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode page response: %v", err)
	}
	if resp.Data.Page != 2 {
		t.Fatalf("page = %d, want 2", resp.Data.Page)
	}
	if resp.Data.PerPage != 2 {
		t.Fatalf("per_page = %d, want 2", resp.Data.PerPage)
	}
	if resp.Data.TotalPages != 2 {
		t.Fatalf("total_pages = %d, want 2", resp.Data.TotalPages)
	}
	if resp.Data.Total != 3 {
		t.Fatalf("total = %d, want 3", resp.Data.Total)
	}
	if len(resp.Data.Results) != 1 {
		t.Fatalf("expected last page to contain 1 result, got %d", len(resp.Data.Results))
	}

	w = performAuthenticatedSearchRequest(t, app, userID, "/api/search/movies?q=Pageable&page=1&per_page=999")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", w.Code, w.Body.String())
	}

	resp = searchMoviesHTTPResponse{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode cap response: %v", err)
	}
	if resp.Data.PerPage != libraryMaxPerPage {
		t.Fatalf("per_page = %d, want cap %d", resp.Data.PerPage, libraryMaxPerPage)
	}
}

// createSearchShow seeds a show the way the scanner does: UpsertLocalShow
// writes the local name, and the TMDB text the index actually searches only
// arrives with UpdateShowMetadata.
func createSearchShow(t *testing.T, app *Application, name, directory, overview, tagline string) int64 {
	t.Helper()

	ctx := context.Background()
	show, err := app.Queries.UpsertLocalShow(ctx, database.UpsertLocalShowParams{
		DirectoryPath: directory,
		LocalName:     name,
		PremiereYear:  sql.NullInt64{Int64: 2024, Valid: true},
		Name:          name,
	})
	if err != nil {
		t.Fatalf("create show %q: %v", name, err)
	}

	err = app.Queries.UpdateShowMetadata(ctx, database.UpdateShowMetadataParams{
		Name:          name,
		Overview:      sql.NullString{String: overview, Valid: overview != ""},
		Tagline:       sql.NullString{String: tagline, Valid: tagline != ""},
		PosterPath:    sql.NullString{String: "/" + directory + ".jpg", Valid: true},
		Certification: sql.NullString{String: "TV-14", Valid: true},
		ID:            show.ID,
	})
	if err != nil {
		t.Fatalf("update show metadata %q: %v", name, err)
	}
	return show.ID
}

func createSearchMusician(t *testing.T, app *Application, name string) int64 {
	t.Helper()

	musicianIdentity, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     name,
		SortName: strings.ToLower(name),
	})
	if err != nil {
		t.Fatalf("create musician %q: %v", name, err)
	}
	return musicianIdentity.ID
}

func createSearchAlbum(t *testing.T, app *Application, title, musician string) int64 {
	t.Helper()

	albumIdentity, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     title,
		SortTitle: strings.ToLower(title),
		Musician:  sql.NullString{String: musician, Valid: true},
	})
	if err != nil {
		t.Fatalf("create album %q: %v", title, err)
	}
	return albumIdentity.ID
}

func createSearchTrack(t *testing.T, app *Application, title, filePath string, albumID, musicianID int64) int64 {
	t.Helper()

	track, err := app.Queries.UpsertTrack(context.Background(), database.UpsertTrackParams{
		Title:         title,
		SortTitle:     strings.ToLower(title),
		FilePath:      filePath,
		FileName:      strings.TrimPrefix(filePath, "/music/"),
		Container:     "flac",
		MimeType:      "audio/flac",
		Codec:         "flac",
		Size:          1,
		TrackIndex:    1,
		Duration:      180,
		Disc:          1,
		Channels:      "2",
		ChannelLayout: "stereo",
		BitRate:       1000,
		Profile:       "",
		AlbumID:       sql.NullInt64{Int64: albumID, Valid: true},
		MusicianID:    sql.NullInt64{Int64: musicianID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create track %q: %v", title, err)
	}
	return track
}

func performAuthenticatedSearchRequest(t *testing.T, app *Application, userID int64, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(newAuthSessionCookie(t, app, userID))

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func TestSearchRoutes_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	userID := createTestUser(t, app, "Search User", "search@example.com", false).ID

	// Search scans into the same row types as the list endpoints, so a hit in
	// every category is what validates those item schemas here.
	createTestMovie(t, app, "Contract Movie", "/movies/search-contract.mkv")
	createSearchShow(t, app, "Contract Show", "/shows/Contract Show (2024)", "A contract show.", "Contract taglines.")
	musicianID := createSearchMusician(t, app, "Contract Artist")
	albumID := createSearchAlbum(t, app, "Contract Album", "Contract Artist")
	createSearchTrack(t, app, "Contract Track", "/music/search-contract.flac", albumID, musicianID)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, userID)

	tests := []struct {
		operationID string
		path        string
		dataKeys    []string
	}{
		{operationID: "searchAll", path: "/api/search?q=contract"},
		{operationID: "searchMovies", path: "/api/search/movies?q=contract", dataKeys: []string{"results"}},
		{operationID: "searchShows", path: "/api/search/shows?q=contract", dataKeys: []string{"results"}},
		{operationID: "searchAlbums", path: "/api/search/albums?q=contract", dataKeys: []string{"results"}},
		{operationID: "searchMusicians", path: "/api/search/musicians?q=contract", dataKeys: []string{"results"}},
		{operationID: "searchTracks", path: "/api/search/tracks?q=contract", dataKeys: []string{"results"}},
	}
	for _, test := range tests {
		t.Run(test.operationID, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			req.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if test.operationID == "searchAll" {
				assertSearchAllSectionsNotEmpty(t, response.Body.Bytes())
			}
			assertResponseListNotEmpty(t, test.operationID, response.Body.Bytes(), test.dataKeys...)
			assertOpenAPIExchange(t, test.operationID, req, response)
		})
	}
}

// searchAll nests its results one level deeper than the category endpoints, so
// it needs its own non-empty check for the section item schemas.
func assertSearchAllSectionsNotEmpty(t *testing.T, body []byte) {
	t.Helper()

	type section struct {
		Results []json.RawMessage `json:"results"`
	}
	var envelope struct {
		Data struct {
			Movies    section `json:"movies"`
			Shows     section `json:"shows"`
			Albums    section `json:"albums"`
			Musicians section `json:"musicians"`
			Tracks    section `json:"tracks"`
		} `json:"data"`
	}
	err := json.Unmarshal(body, &envelope)
	if err != nil {
		t.Fatalf("searchAll: decode body: %v", err)
	}
	for name, results := range map[string][]json.RawMessage{
		"movies":    envelope.Data.Movies.Results,
		"shows":     envelope.Data.Shows.Results,
		"albums":    envelope.Data.Albums.Results,
		"musicians": envelope.Data.Musicians.Results,
		"tracks":    envelope.Data.Tracks.Results,
	} {
		if len(results) == 0 {
			t.Fatalf("searchAll returned an empty %s section, so its item schema would not be validated: %s", name, body)
		}
	}
}

func TestNormalizeSearchPage(t *testing.T) {
	tests := []struct {
		name      string
		page      int64
		total     int64
		perPage   int64
		wantPage  int64
		wantPages int64
	}{
		{
			name:      "keeps in range page",
			page:      2,
			total:     50,
			perPage:   24,
			wantPage:  2,
			wantPages: 3,
		},
		{
			name:      "clamps overlarge page",
			page:      999,
			total:     50,
			perPage:   24,
			wantPage:  3,
			wantPages: 3,
		},
		{
			name:      "empty result resets to first page",
			page:      999,
			total:     0,
			perPage:   24,
			wantPage:  1,
			wantPages: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPages := normalizeSearchPage(tt.page, tt.total, tt.perPage)
			if gotPage != tt.wantPage {
				t.Fatalf("normalizeSearchPage page = %d, want %d", gotPage, tt.wantPage)
			}
			if gotPages != tt.wantPages {
				t.Fatalf("normalizeSearchPage pages = %d, want %d", gotPages, tt.wantPages)
			}
		})
	}
}
