package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
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
	Error   bool             `json:"error"`
	Message string           `json:"message,omitempty"`
	Data    searchMoviesData `json:"data"`
}

func TestBuildFTSQuery(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{
			name: "simple prefix tokens",
			raw:  "Casino Royale",
			want: "casino* OR royale*",
			ok:   true,
		},
		{
			name: "keeps unicode letters for unicode61 tokenizer",
			raw:  "Beyoncé año",
			want: "beyoncé* OR año*",
			ok:   true,
		},
		{
			name: "strips fts syntax",
			raw:  `"casino" OR title:royale`,
			want: "casino* OR or* OR title* OR royale*",
			ok:   true,
		},
		{
			name: "punctuation only",
			raw:  `"'():`,
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := buildFTSQuery(tt.raw)
			if ok != tt.ok {
				t.Fatalf("buildFTSQuery(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("buildFTSQuery(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSearchMoviesBroadMatchingRanksExactTitleFirst(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	createSearchMovie(t, app, "Casino Nights", "/movies/casino-nights.mkv")
	createSearchMovie(t, app, "Royale Tenenbaums", "/movies/royale-tenenbaums.mkv")
	createSearchMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	match, ok := buildFTSQuery("Casino Royale")
	if !ok {
		t.Fatal("expected usable FTS query")
	}

	results, err := app.searchMovies(context.Background(), "Casino Royale", match, 10, 0)
	if err != nil {
		t.Fatalf("searchMovies failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected broad search to return 3 movies, got %d", len(results))
	}
	if results[0].Title != "Casino Royale" {
		t.Fatalf("expected exact title first, got %q", results[0].Title)
	}
}

func TestSearchMoviesFTSSyntaxInputDoesNotSuppressResults(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	createSearchMovie(t, app, "Casino Royale", "/movies/casino-royale.mkv")

	match, ok := buildFTSQuery(`"Casino" OR title:royale`)
	if !ok {
		t.Fatal("expected usable FTS query")
	}

	results, err := app.searchMovies(context.Background(), `"Casino" OR title:royale`, match, 10, 0)
	if err != nil {
		t.Fatalf("searchMovies failed: %v", err)
	}
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
			match, ok := buildFTSQuery(query)
			if !ok {
				t.Fatal("expected usable FTS query")
			}

			results, err := app.searchTracks(context.Background(), query, match, 10, 0)
			if err != nil {
				t.Fatalf("searchTracks failed: %v", err)
			}
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

	match, ok := buildFTSQuery("Sia")
	if !ok {
		t.Fatal("expected usable FTS query")
	}

	results, err := app.searchTracks(context.Background(), "Sia", match, 10, 0)
	if err != nil {
		t.Fatalf("searchTracks before update failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no Sia results before update, got %#v", results)
	}

	createSearchTrack(t, app, "Hello", "/music/hello.flac", albumID, updatedMusicianID)

	results, err = app.searchTracks(context.Background(), "Sia", match, 10, 0)
	if err != nil {
		t.Fatalf("searchTracks after update failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Hello" {
		t.Fatalf("expected updated track relationship to be searchable, got %#v", results)
	}

	match, ok = buildFTSQuery("Adele")
	if !ok {
		t.Fatal("expected usable FTS query")
	}

	results, err = app.searchTracks(context.Background(), "Adele", match, 10, 0)
	if err != nil {
		t.Fatalf("searchTracks old relationship failed: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected old musician relationship to be removed from search, got %#v", results)
	}
}

func TestInitTablesRebuildsSearchIndexesForExistingRows(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	app := &Application{DB: db}
	setupTestLogger(t, app)

	schemaBeforeFTS, _, ok := strings.Cut(SQL, "-- FTS5 virtual tables")
	if !ok {
		t.Fatal("expected schema to contain FTS marker")
	}
	_, err = db.Exec(schemaBeforeFTS)
	if err != nil {
		t.Fatalf("create pre-FTS schema: %v", err)
	}
	insertPreFTSSearchMovie(t, db, "Preexisting Movie", "/movies/preexisting.mkv")

	err = app.InitTables()
	if err != nil {
		t.Fatalf("InitTables failed: %v", err)
	}

	match, ok := buildFTSQuery("Preexisting")
	if !ok {
		t.Fatal("expected usable FTS query")
	}

	results, err := app.searchMovies(context.Background(), "Preexisting", match, 10, 0)
	if err != nil {
		t.Fatalf("searchMovies failed: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Preexisting Movie" {
		t.Fatalf("expected rebuilt FTS result, got %#v", results)
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
	if resp.Data.PerPage != helpers.SEARCH_MAX_PER_PAGE {
		t.Fatalf("per_page = %d, want cap %d", resp.Data.PerPage, helpers.SEARCH_MAX_PER_PAGE)
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

func insertPreFTSSearchMovie(t *testing.T, db *sql.DB, title, filePath string) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO movies (title, file_path, file_name, size, container, mime_type, adult)
		VALUES (?, ?, ?, 1, 'mkv', 'video/x-matroska', 0)
	`, title, filePath, strings.TrimPrefix(filePath, "/movies/"))
	if err != nil {
		t.Fatalf("insert movie %q: %v", title, err)
	}
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

func searchAuthCookies(t *testing.T, app *Application, userID int64) []*http.Cookie {
	t.Helper()

	handler := app.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
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
