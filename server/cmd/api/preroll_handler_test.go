package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/preroll"
	"igloo/cmd/internal/tmdb"
)

func prerollPath(movieID int64) string {
	return fmt.Sprintf("/api/movies/%d/preroll", movieID)
}

func decodePreroll(t *testing.T, body string) []prerollTrailerResponse {
	t.Helper()

	var envelope struct {
		Error bool `json:"error"`
		Data  struct {
			Trailers []prerollTrailerResponse `json:"trailers"`
		} `json:"data"`
	}
	err := json.Unmarshal([]byte(body), &envelope)
	if err != nil {
		t.Fatalf("decode preroll: %v; body = %s", err, body)
	}
	if envelope.Error {
		t.Fatalf("unexpected error envelope: %s", body)
	}
	if envelope.Data.Trailers == nil {
		t.Fatalf("trailers must be a list, never null: %s", body)
	}
	return envelope.Data.Trailers
}

// createTrailerMovie stores a movie with an optional TMDB id and one YouTube
// trailer, returning the movie id.
func createTrailerMovie(t *testing.T, app *Application, title string, tmdbID int64, key string) int64 {
	t.Helper()
	ctx := context.Background()

	movieID, err := app.Queries.UpsertMovie(ctx, database.UpsertMovieParams{
		Title:     title,
		FilePath:  "/media/" + key + ".mkv",
		FileName:  key + ".mkv",
		Size:      1,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
		TmdbID:    helpers.NullInt64(tmdbID),
	})
	if err != nil {
		t.Fatalf("create movie %q: %v", title, err)
	}
	if key == "" {
		return movieID
	}
	videoID, err := app.Queries.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{
		Title:      "Official Trailer",
		ExternalID: helpers.NullString("tmdb-video-" + key),
		Key:        key,
		Type:       "trailer",
		Site:       "youtube",
	})
	if err != nil {
		t.Fatalf("create extra video: %v", err)
	}
	err = app.Queries.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{MovieID: movieID, ExtraVideoID: videoID})
	if err != nil {
		t.Fatalf("link extra video: %v", err)
	}
	return movieID
}

func savePrerollPrefs(t *testing.T, app *Application, userID int64, enabled bool, count int, source string) {
	t.Helper()
	_, err := app.Queries.UpsertUserPreferences(context.Background(), database.UpsertUserPreferencesParams{
		UserID:          userID,
		TrailersEnabled: enabled,
		TrailersCount:   int64(count),
		TrailersSource:  source,
	})
	if err != nil {
		t.Fatalf("save preferences: %v", err)
	}
}

// countingTheaterSource counts now-playing lookups; the pool refreshes on its
// own goroutine, hence the atomic.
type countingTheaterSource struct {
	preroll.TheaterSource
	listCalls atomic.Int32
}

func (s *countingTheaterSource) GetMoviesInTheaters(ctx context.Context) ([]*tmdb.TmdbMovie, error) {
	s.listCalls.Add(1)
	return s.TheaterSource.GetMoviesInTheaters(ctx)
}

func theaterStub(ids ...int) *stubTmdbClient {
	stub := &stubTmdbClient{detailMovies: map[int]tmdb.TmdbMovie{}}
	for _, id := range ids {
		listed := &tmdb.TmdbMovie{TmdbID: id, Title: fmt.Sprintf("Theater %d", id)}
		stub.theaterMovies = append(stub.theaterMovies, listed)
		details := tmdb.TmdbMovie{TmdbID: id, Title: listed.Title}
		details.Videos.Results = []tmdb.TmdbVideoResult{
			{ID: fmt.Sprintf("t%d", id), Key: fmt.Sprintf("theater-%d", id), Site: "YouTube", Type: "Trailer", Official: true},
		}
		stub.detailMovies[id] = details
	}
	return stub
}

func TestGetMoviePreroll_DisabledReturnsEmptyList(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	playing := createTrailerMovie(t, app, "Feature", 1, "feature")
	createTrailerMovie(t, app, "Other", 2, "other")
	app.InitRouter()

	req := newOpenAPIJSONRequest(http.MethodGet, prerollPath(playing), "")
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	resp := serveOpenAPIExchange(t, app.Router, "getMoviePreroll", req, http.StatusOK)
	if trailers := decodePreroll(t, resp.Body.String()); len(trailers) != 0 {
		t.Fatalf("disabled feature returned %+v", trailers)
	}
}

func TestGetMoviePreroll_LibraryTrailersExcludeTheMovieItself(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	playing := createTrailerMovie(t, app, "Feature", 1, "feature")
	otherA := createTrailerMovie(t, app, "Other A", 2, "other-a")
	otherB := createTrailerMovie(t, app, "Other B", 0, "other-b")
	createTrailerMovie(t, app, "No trailer", 4, "")
	savePrerollPrefs(t, app, user.ID, true, 5, preroll.SourceLibrary)

	resp := serveAs(t, app, user.ID, http.MethodGet, prerollPath(playing), "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.Code, resp.Body.String())
	}
	trailers := decodePreroll(t, resp.Body.String())
	if len(trailers) != 2 {
		t.Fatalf("trailers = %+v, want the two other movies", trailers)
	}
	seen := map[int64]prerollTrailerResponse{}
	for _, trailer := range trailers {
		if trailer.Source != preroll.SourceLibrary || trailer.MovieID == nil {
			t.Fatalf("library trailer shape = %+v", trailer)
		}
		if *trailer.MovieID == playing {
			t.Fatal("the movie about to play was queued")
		}
		seen[*trailer.MovieID] = trailer
	}
	if seen[otherA].TmdbID == nil || *seen[otherA].TmdbID != 2 || seen[otherA].Title != "Other A" || seen[otherA].YouTubeKey != "other-a" {
		t.Fatalf("other A = %+v", seen[otherA])
	}
	if seen[otherB].TmdbID != nil {
		t.Fatalf("a movie without a TMDB id must report null, got %+v", seen[otherB])
	}
}

func TestGetMoviePreroll_TheatersPreferenceFallsBackToLibraryWithoutTmdb(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	playing := createTrailerMovie(t, app, "Feature", 1, "feature")
	createTrailerMovie(t, app, "Other", 2, "other")
	savePrerollPrefs(t, app, user.ID, true, 2, preroll.SourceTheaters)
	if app.Tmdb != nil {
		t.Fatal("test expects an app without TMDB")
	}

	resp := serveAs(t, app, user.ID, http.MethodGet, prerollPath(playing), "")
	trailers := decodePreroll(t, resp.Body.String())
	if len(trailers) != 1 || trailers[0].Source != preroll.SourceLibrary {
		t.Fatalf("want the library top-up, got %+v", trailers)
	}
}

func TestGetMoviePreroll_TheatersFirstThenLibrary(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	// The feature is itself in theaters (TMDB 10) and so is library movie 20.
	playing := createTrailerMovie(t, app, "Feature", 10, "feature")
	createTrailerMovie(t, app, "Shared", 20, "shared")
	createTrailerMovie(t, app, "Library only", 0, "library-only")
	stub := theaterStub(10, 20, 30)
	app.Tmdb = stub
	app.PrerollTheaters = preroll.NewTheatersPool(stub, app.Logger)
	savePrerollPrefs(t, app, user.ID, true, 4, preroll.SourceBoth)

	resp := serveAs(t, app, user.ID, http.MethodGet, prerollPath(playing), "")
	trailers := decodePreroll(t, resp.Body.String())
	if len(trailers) != 3 {
		t.Fatalf("trailers = %+v, want 3 (two theaters, one library)", trailers)
	}
	if trailers[0].Source != preroll.SourceTheaters || trailers[1].Source != preroll.SourceTheaters || trailers[2].Source != preroll.SourceLibrary {
		t.Fatalf("order = %+v, want theaters, theaters, library", trailers)
	}
	for _, trailer := range trailers[:2] {
		if trailer.TmdbID == nil || *trailer.TmdbID == 10 || trailer.MovieID != nil {
			t.Fatalf("theaters trailer = %+v", trailer)
		}
	}
	if trailers[2].Title != "Library only" {
		t.Fatalf("the library share must skip TMDB 20, already queued from theaters: %+v", trailers[2])
	}
}

func TestGetMoviePreroll_LibraryPreferenceTopsUpFromTheaters(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	playing := createTrailerMovie(t, app, "Feature", 10, "feature")
	createTrailerMovie(t, app, "Other", 20, "other")
	stub := theaterStub(10, 30, 40)
	app.Tmdb = stub
	app.PrerollTheaters = preroll.NewTheatersPool(stub, app.Logger)
	savePrerollPrefs(t, app, user.ID, true, 3, preroll.SourceLibrary)

	resp := serveAs(t, app, user.ID, http.MethodGet, prerollPath(playing), "")
	trailers := decodePreroll(t, resp.Body.String())
	if len(trailers) != 3 {
		t.Fatalf("trailers = %+v, want the library trailer topped up with two theaters trailers", trailers)
	}
	if trailers[0].Source != preroll.SourceTheaters || trailers[1].Source != preroll.SourceTheaters || trailers[2].Source != preroll.SourceLibrary {
		t.Fatalf("order = %+v, want theaters, theaters, library", trailers)
	}
	for _, trailer := range trailers[:2] {
		if trailer.TmdbID == nil || *trailer.TmdbID == 10 {
			t.Fatalf("the top-up must skip the movie about to play: %+v", trailer)
		}
	}
}

func TestGetMoviePreroll_FilledLibraryPreferenceSkipsTheaters(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	playing := createTrailerMovie(t, app, "Feature", 10, "feature")
	createTrailerMovie(t, app, "Other A", 20, "other-a")
	createTrailerMovie(t, app, "Other B", 30, "other-b")
	stub := theaterStub(40, 50)
	source := &countingTheaterSource{TheaterSource: stub}
	app.Tmdb = stub
	app.PrerollTheaters = preroll.NewTheatersPool(source, app.Logger)
	savePrerollPrefs(t, app, user.ID, true, 2, preroll.SourceLibrary)

	resp := serveAs(t, app, user.ID, http.MethodGet, prerollPath(playing), "")
	trailers := decodePreroll(t, resp.Body.String())
	if len(trailers) != 2 || trailers[0].Source != preroll.SourceLibrary || trailers[1].Source != preroll.SourceLibrary {
		t.Fatalf("trailers = %+v, want two library trailers", trailers)
	}
	if calls := source.listCalls.Load(); calls != 0 {
		t.Fatalf("a library preference the library fills must not resolve theaters (%d calls)", calls)
	}
}

func TestGetMoviePreroll_UnknownMovieAndBadID(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	savePrerollPrefs(t, app, user.ID, true, 2, preroll.SourceBoth)

	resp := serveAs(t, app, user.ID, http.MethodGet, prerollPath(999), "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("unknown movie status = %d: %s", resp.Code, resp.Body.String())
	}
	resp = serveAs(t, app, user.ID, http.MethodGet, "/api/movies/abc/preroll", "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("bad id status = %d: %s", resp.Code, resp.Body.String())
	}
	resp = serveAs(t, app, 0, http.MethodGet, prerollPath(1), "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", resp.Code)
	}
}

func TestGetMoviePreroll_AcceptsDeviceTokens(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "TV Viewer", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "Living Room TV", "android_tv")
	playing := createTrailerMovie(t, app, "Feature", 1, "feature")
	createTrailerMovie(t, app, "Other", 2, "other")
	savePrerollPrefs(t, app, user.ID, true, 1, preroll.SourceLibrary)
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, prerollPath(playing), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	app.Router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("device status = %d: %s", resp.Code, resp.Body.String())
	}
	if trailers := decodePreroll(t, resp.Body.String()); len(trailers) != 1 || trailers[0].YouTubeKey != "other" {
		t.Fatalf("device trailers = %+v", trailers)
	}
}
