package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
	"igloo/cmd/internal/scanner/scannertest"
)

// The whole way a downloaded image goes: turning the setting on starts a
// pass, the pass stores the copy in the static directory and points the movie
// at it, and the API serves the copy at the URL the movie returns. Turning it
// off returns the TMDB path as soon as the save answers, and a pass then
// deletes the copy.
func TestDownloadImages_ServesTheMoviePosterFromTheStaticDirectory(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Settings Admin", "settings-admin@example.com", true)
	poster := []byte("\x89PNG\r\n\x1a\nthe poster")
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/"+helpers.TMDB_POSTER_SIZE+"/poster.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Write(poster)
	}))
	t.Cleanup(upstream.Close)
	app.TmdbImageBaseURL = upstream.URL
	app.TmdbImageHTTPClient = upstream.Client()
	// The syncer takes the client when it is built.
	app.initArtwork()

	movieID, err := app.Queries.UpsertMovie(t.Context(), database.UpsertMovieParams{
		Title: "The Matrix", FilePath: "/movies/matrix.mkv", FileName: "matrix.mkv", Size: 1,
		Container: "mkv", MimeType: "video/x-matroska",
		TmdbID: helpers.NullInt64(603), PosterPathSource: helpers.NullString("/poster.jpg"),
	})
	if err != nil {
		t.Fatal(err)
	}
	staticDir := app.CurrentSettings().StaticDir
	moviePoster := func() string {
		t.Helper()
		w := serveAs(t, app, admin.ID, http.MethodGet, "/api/movies/details/"+strconv.FormatInt(movieID, 10), "")
		if w.Code != http.StatusOK {
			t.Fatalf("movie details: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Data struct {
				Movie struct {
					PosterPath struct{ String string } `json:"poster_path"`
				} `json:"movie"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &body)
		if err != nil {
			t.Fatal(err)
		}
		return body.Data.Movie.PosterPath.String
	}
	saveDownloadImages := func(on bool) {
		t.Helper()
		body := generalSettingsBodyWithWatcher(staticDir, false)
		if !on {
			body = strings.Replace(body, `"download_images": true`, `"download_images": false`, 1)
		}
		w := serveAs(t, app, admin.ID, http.MethodPut, "/api/settings/general", body)
		if w.Code != http.StatusOK {
			t.Fatalf("save settings: %d %s", w.Code, w.Body.String())
		}
	}
	waitForPass := func() {
		t.Helper()
		scannertest.WaitForGroup(t, app.Wait, 10*time.Second, "the artwork pass")
	}

	saveDownloadImages(true)
	waitForPass()

	served := moviePoster()
	if !strings.HasPrefix(served, "/api/static/movies/603/the_matrix_poster.png?v=") {
		t.Fatalf("poster_path = %q, want the downloaded copy", served)
	}
	w := serveAs(t, app, admin.ID, http.MethodGet, served, "")
	if w.Code != http.StatusOK || w.Body.String() != string(poster) || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("GET %s = %d %q (%s), want the poster", served, w.Code, w.Body.String(), w.Header().Get("Content-Type"))
	}
	if requests.Load() != 1 {
		t.Fatalf("TMDB requests = %d, want one", requests.Load())
	}

	saveDownloadImages(false)
	if moviePoster() != "/poster.jpg" {
		t.Fatalf("poster_path = %q right after turning downloads off, want the TMDB path", moviePoster())
	}
	waitForPass()
	_, err = os.Stat(filepath.Join(staticDir, "movies"))
	if !os.IsNotExist(err) {
		t.Fatalf("the movies folder is still there: %v", err)
	}
}
