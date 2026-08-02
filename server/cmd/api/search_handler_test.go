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
	defer app.DB.Close()

	createSearchMovie(t, app, "Casino Nights", "/movies/casino-nights.mkv")
	createSearchMovie(t, app, "Royale Tenenbaums", "/movies/royale-tenenbaums.mkv")
	createSearchMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

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

func TestSearchMoviesTypoInOneTokenRanksTargetFirst(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	createSearchMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")
	createSearchMovie(t, app, "Kill Bill: Volume 1", "/movies/kill-bill-1.mkv")
	createSearchMovie(t, app, "A Time to Kill", "/movies/a-time-to-kill.mkv")

	results := searchEntityResults(t, app, movieSearchEntity, "License to Kill")
	if len(results) == 0 {
		t.Fatal("expected typo-corrected search to return results")
	}
	if results[0].Title != "Licence to Kill" {
		t.Fatalf("expected Licence to Kill first, got %q", results[0].Title)
	}
}

func TestSearchMoviesSingleTokenTypoReturnsResult(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	createSearchMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")

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

func TestSearchTracksMusicianTypoReturnsResult(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

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
	defer app.DB.Close()

	createSearchMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	results := searchEntityResults(t, app, movieSearchEntity, `"Casino" OR title:royale`)
	if len(results) == 0 || results[0].Title != "Casino Royale" {
		t.Fatalf("expected Casino Royale result, got %#v", results)
	}
}

func TestSearchTracksMatchesTrackAlbumAndArtist(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

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
	defer app.DB.Close()

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
	defer app.DB.Close()

	userID := createSearchUser(t, app)
	createSearchMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

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
	defer app.DB.Close()

	userID := createSearchUser(t, app)
	createSearchMovie(t, app, "Licence to Kill", "/movies/licence-to-kill.mkv")
	createSearchMovie(t, app, "Kill Bill: Volume 1", "/movies/kill-bill-1.mkv")

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
	defer app.DB.Close()

	userID := createSearchUser(t, app)
	createSearchMovie(t, app, "Pageable Movie One", "/movies/pageable-1.mkv")
	createSearchMovie(t, app, "Pageable Movie Two", "/movies/pageable-2.mkv")
	createSearchMovie(t, app, "Pageable Movie Three", "/movies/pageable-3.mkv")

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
	if resp.Data.PerPage != searchMaxPerPage {
		t.Fatalf("per_page = %d, want cap %d", resp.Data.PerPage, searchMaxPerPage)
	}
}

func createSearchUser(t *testing.T, app *Application) int64 {
	t.Helper()

	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     "Search User",
		Email:    "search@example.com",
		Password: "hashed",
		IsAdmin:  false,
	})
	if err != nil {
		t.Fatalf("create search user: %v", err)
	}
	return user.ID
}

func createSearchMovie(t *testing.T, app *Application, title, filePath string) int64 {
	t.Helper()

	movie, err := app.Queries.UpsertMovie(context.Background(), database.UpsertMovieParams{
		Title:     title,
		FilePath:  filePath,
		FileName:  strings.TrimPrefix(filePath, "/movies/"),
		Size:      1,
		Container: "mkv",
		MimeType:  "video/x-matroska",
		Adult:     false,
	})
	if err != nil {
		t.Fatalf("create movie %q: %v", title, err)
	}
	return movie.ID
}

func createSearchMusician(t *testing.T, app *Application, name string) int64 {
	t.Helper()

	musician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
		Name:     name,
		SortName: strings.ToLower(name),
	})
	if err != nil {
		t.Fatalf("create musician %q: %v", name, err)
	}
	return musician.ID
}

func createSearchAlbum(t *testing.T, app *Application, title, musician string) int64 {
	t.Helper()

	album, err := app.Queries.UpsertAlbum(context.Background(), database.UpsertAlbumParams{
		Title:     title,
		SortTitle: strings.ToLower(title),
		Musician:  sql.NullString{String: musician, Valid: true},
	})
	if err != nil {
		t.Fatalf("create album %q: %v", title, err)
	}
	return album.ID
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
	return track.ID
}

func performAuthenticatedSearchRequest(t *testing.T, app *Application, userID int64, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	for _, cookie := range searchAuthCookies(t, app, userID) {
		req.AddCookie(cookie)
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func TestSearchRoutes_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	userID := createSearchUser(t, app)
	app.InitSession()
	app.InitRouter()
	cookies := searchAuthCookies(t, app, userID)

	tests := []struct {
		operationID string
		path        string
	}{
		{operationID: "searchAll", path: "/api/search?q=contract"},
		{operationID: "searchMovies", path: "/api/search/movies?q=contract"},
		{operationID: "searchAlbums", path: "/api/search/albums?q=contract"},
		{operationID: "searchMusicians", path: "/api/search/musicians?q=contract"},
		{operationID: "searchTracks", path: "/api/search/tracks?q=contract"},
	}
	for _, test := range tests {
		t.Run(test.operationID, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			assertOpenAPIExchange(t, test.operationID, req, response)
		})
	}
}

func searchAuthCookies(t *testing.T, app *Application, userID int64) []*http.Cookie {
	t.Helper()

	handler := app.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	return resp.Cookies()
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
