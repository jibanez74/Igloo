package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

// listedMovieTitles returns the titles a movie list route serves, in order.
func listedMovieTitles(t *testing.T, handler http.Handler, target string) []string {
	t.Helper()

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("%s status = %d: %s", target, response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Movies []struct {
				Title string `json:"title"`
			} `json:"movies"`
		} `json:"data"`
	}
	err := json.Unmarshal(response.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("decode %s: %v", target, err)
	}
	titles := make([]string, 0, len(body.Data.Movies))
	for _, movie := range body.Data.Movies {
		titles = append(titles, movie.Title)
	}
	return titles
}

// Every movie list accepts ?sort=desc, which runs a separate query whose rows
// are mapped back onto the ascending row type; the mapping must keep the rows
// intact and the order reversed.
func TestMovieLists_SortDescendingReversesTheAscendingOrder(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()
	user := createTestUser(t, app, "Sorter", "sorter@example.com", false)

	movieIDs := make([]int64, 0, 3)
	for i, title := range []string{"beta", "Alpha", "gamma"} {
		movieIDs = append(movieIDs, createTestMovie(t, app, title, fmt.Sprintf("/movies/sort-%d.mkv", i)))
	}
	genreID := createMovieGenre(t, app, movieIDs[0], "Sorted Genre")
	playlistID := createMoviePlaylist(t, app, user.ID, movieIDs[0])
	for i, movieID := range movieIDs {
		if i > 0 {
			err := app.Queries.CreateMovieGenre(ctx, database.CreateMovieGenreParams{MovieID: movieID, GenreID: genreID})
			if err != nil {
				t.Fatalf("link genre: %v", err)
			}
			_, err = app.DB.Exec("INSERT INTO playlist_movies (playlist_id, movie_id, position, added_by) VALUES (?, ?, ?, ?)", playlistID, movieID, i+1, user.ID)
			if err != nil {
				t.Fatalf("add movie to playlist: %v", err)
			}
		}
		_, err := app.DB.Exec("INSERT INTO user_liked_movies (user_id, movie_id) VALUES (?, ?)", user.ID, movieID)
		if err != nil {
			t.Fatalf("like movie: %v", err)
		}
	}
	handler := authenticatedRouter(t, app, user.ID)

	routes := map[string]string{
		"library":  "/api/movies/library",
		"liked":    "/api/movies/liked",
		"genre":    fmt.Sprintf("/api/movies/genres/%d/movies", genreID),
		"playlist": fmt.Sprintf("/api/movies/playlists/%d/movies", playlistID),
	}
	for name, path := range routes {
		t.Run(name, func(t *testing.T) {
			ascending := listedMovieTitles(t, handler, path+"?sort=asc")
			descending := listedMovieTitles(t, handler, path+"?sort=desc")
			if len(ascending) != 3 {
				t.Fatalf("ascending titles = %v, want all three movies", ascending)
			}
			slices.Reverse(ascending)
			if !slices.Equal(descending, ascending) {
				t.Fatalf("descending titles = %v, want the ascending order reversed %v", descending, ascending)
			}
		})
	}
}

// A bulk scan adds many movies within one second of created_at; the latest
// row breaks those ties by id so the newest insert still comes first.
// The home billboard features the newest movie when nothing is in progress,
// so the latest row carries each movie's backdrop and overview.
func TestGetLatestMovies_CarriesBackdropAndOverview(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "latest-billboard@example.com", false)
	withArt := createTestMovie(t, app, "With Art", "/movies/latest-with-art.mkv")
	bare := createTestMovie(t, app, "Bare", "/movies/latest-bare.mkv")
	_, err := app.DB.Exec(
		`UPDATE movies SET backdrop_path_source = '/latest-backdrop.jpg', overview = 'Newly added.' WHERE id = ?`,
		withArt,
	)
	if err != nil {
		t.Fatal(err)
	}

	w := serveAs(t, app, user.ID, http.MethodGet, "/api/movies/latest", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Movies []struct {
				ID           int64          `json:"id"`
				BackdropPath sql.NullString `json:"backdrop_path"`
				Overview     sql.NullString `json:"overview"`
			} `json:"movies"`
		} `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}

	byID := map[int64]int{}
	for index, movie := range resp.Data.Movies {
		byID[movie.ID] = index
	}
	got := resp.Data.Movies[byID[withArt]]
	if want := (sql.NullString{String: "/latest-backdrop.jpg", Valid: true}); got.BackdropPath != want {
		t.Errorf("backdrop = %+v, want %+v", got.BackdropPath, want)
	}
	if want := (sql.NullString{String: "Newly added.", Valid: true}); got.Overview != want {
		t.Errorf("overview = %+v, want %+v", got.Overview, want)
	}
	// A movie with neither still sends both keys, as nulls.
	if !strings.Contains(w.Body.String(), `"backdrop_path":{"String":"","Valid":false}`) {
		t.Errorf("a movie without a backdrop must send a null backdrop_path: %s", w.Body.String())
	}
	if none := resp.Data.Movies[byID[bare]]; none.BackdropPath.Valid {
		t.Errorf("bare movie backdrop = %+v, want null", none.BackdropPath)
	}
}

func TestGetLatestMovies_BreaksSameSecondTiesByNewestID(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Viewer", "latest-viewer@example.com", false)
	var ids []int64
	for i := range 3 {
		ids = append(ids, createTestMovie(t, app, fmt.Sprintf("Latest %d", i), fmt.Sprintf("/movies/latest-%d.mkv", i)))
	}
	_, err := app.DB.Exec(`UPDATE movies SET created_at = '2026-01-01 00:00:00'`)
	if err != nil {
		t.Fatal(err)
	}

	w := serveAs(t, app, user.ID, http.MethodGet, "/api/movies/latest", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Movies []struct {
				ID int64 `json:"id"`
			} `json:"movies"`
		} `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int64, 0, len(resp.Data.Movies))
	for _, movie := range resp.Data.Movies {
		got = append(got, movie.ID)
	}
	want := []int64{ids[2], ids[1], ids[0]}
	if !slices.Equal(got, want) {
		t.Fatalf("latest ids = %v, want %v", got, want)
	}
}

func seedStreamTestMovie(t *testing.T, app *Application, container, mimeType string, content []byte) database.Movie {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "Stream Test (2024)."+container)
	err := os.WriteFile(path, content, 0o644)
	if err != nil {
		t.Fatalf("write movie file: %v", err)
	}

	movieID, err := app.Queries.UpsertMovie(context.Background(), database.UpsertMovieParams{
		Title:     "Stream Test",
		FilePath:  path,
		FileName:  filepath.Base(path),
		Size:      int64(len(content)),
		Container: container,
		MimeType:  mimeType,
	})
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	movie, err := app.Queries.GetMovieByID(context.Background(), movieID)
	if err != nil {
		t.Fatal(err)
	}
	return movie
}

func streamTestHandler(t *testing.T, app *Application) http.Handler {
	t.Helper()
	viewer := createTestUser(t, app, "Viewer", "viewer@example.com", false)
	return authenticatedRouter(t, app, viewer.ID)
}

func TestStreamMovieDerivesContentTypeFromContainer(t *testing.T) {
	app := setupTestApp(t)

	// A pre-fix row scanned on a host without /etc/mime.types stored
	// application/octet-stream; the handler must still answer from the
	// pinned container map.
	movie := seedStreamTestMovie(t, app, "mkv", "application/octet-stream", []byte("matroska"))
	handler := streamTestHandler(t, app)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "video/x-matroska" {
		t.Errorf("Content-Type = %q, want video/x-matroska", got)
	}
}

// The bounded, conditional and unsatisfiable range forms are exercised for
// every media route by TestMediaResponsesConformToOpenAPI; the open-ended and
// suffix forms are only sent here.
func TestStreamMovieServesRanges(t *testing.T) {
	app := setupTestApp(t)

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(t, app)
	size := len(content)

	cases := []struct {
		name        string
		rangeHeader string
		wantStatus  int
		wantRange   string
		wantBody    []byte
	}{
		{
			name:        "open-ended range",
			rangeHeader: fmt.Sprintf("bytes=%d-", size-50),
			wantStatus:  http.StatusPartialContent,
			wantRange:   fmt.Sprintf("bytes %d-%d/%d", size-50, size-1, size),
			wantBody:    content[size-50:],
		},
		{
			name:        "suffix range",
			rangeHeader: "bytes=-100",
			wantStatus:  http.StatusPartialContent,
			wantRange:   fmt.Sprintf("bytes %d-%d/%d", size-100, size-1, size),
			wantBody:    content[size-100:],
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
			req.Header.Set("Range", tc.rangeHeader)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
			if got := w.Header().Get("Content-Range"); got != tc.wantRange {
				t.Errorf("Content-Range = %q, want %q", got, tc.wantRange)
			}
			if tc.wantBody != nil && !bytes.Equal(w.Body.Bytes(), tc.wantBody) {
				t.Errorf("body = %d bytes, want the exact %d requested bytes", w.Body.Len(), len(tc.wantBody))
			}
		})
	}
}

// TestMediaResponsesConformToOpenAPI proves an ETag is present and honoured;
// this pins the format the validator is derived from.
func TestStreamMovieETagFormat(t *testing.T) {
	app := setupTestApp(t)

	content := bytes.Repeat([]byte("0123456789"), 30)
	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", content)
	handler := streamTestHandler(t, app)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/stream", movie.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	stat, err := os.Stat(movie.FilePath)
	if err != nil {
		t.Fatalf("stat movie file: %v", err)
	}
	want := fmt.Sprintf("\"%x-%x\"", stat.Size(), stat.ModTime().UnixNano())
	if etag := w.Header().Get("ETag"); etag != want {
		t.Errorf("ETag = %q, want %q", etag, want)
	}
}

func TestStreamMovieErrorPaths(t *testing.T) {
	app := setupTestApp(t)

	movie := seedStreamTestMovie(t, app, "mp4", "video/mp4", []byte("gone soon"))
	err := os.Remove(movie.FilePath)
	if err != nil {
		t.Fatalf("remove movie file: %v", err)
	}
	handler := streamTestHandler(t, app)

	cases := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{name: "invalid id", target: "/api/movies/not-a-number/stream", wantStatus: http.StatusBadRequest},
		{name: "missing row", target: "/api/movies/999999/stream", wantStatus: http.StatusNotFound},
		{name: "file gone", target: fmt.Sprintf("/api/movies/%d/stream", movie.ID), wantStatus: http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tc.wantStatus)
			}
		})
	}
}

func TestToggleLikeMovie_HTTPPersistsLikeAndUnlike(t *testing.T) {
	app := setupSessionTestApp(t)

	userID, movieID := createTestUserAndMovie(t, app)

	handler := authenticatedRouter(t, app, userID)

	postLike := func(t *testing.T) bool {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/movies/%d/like", movieID), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
		}
		assertOpenAPIExchange(t, "toggleLikeMovie", req, w)

		var resp struct {
			Error bool `json:"error"`
			Data  struct {
				MovieID int64 `json:"movie_id"`
				IsLiked bool  `json:"is_liked"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.Error {
			t.Fatalf("expected success response, got %s", w.Body.String())
		}
		if resp.Data.MovieID != movieID {
			t.Fatalf("movie_id=%d, want %d", resp.Data.MovieID, movieID)
		}

		return resp.Data.IsLiked
	}

	if isLiked := postLike(t); !isLiked {
		t.Fatal("first like toggle returned is_liked=false, want true")
	}
	statusReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/movies/%d/like-status", movieID), nil)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, statusReq)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("like status=%d, body=%s", statusResponse.Code, statusResponse.Body.String())
	}
	assertOpenAPIExchange(t, "getMovieLikeStatus", statusReq, statusResponse)
	persistedLike, err := app.Queries.IsMovieLiked(context.Background(), database.IsMovieLikedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("IsMovieLiked after like: %v", err)
	}
	if !persistedLike {
		t.Fatal("expected liked row after first toggle")
	}

	if isLiked := postLike(t); isLiked {
		t.Fatal("second like toggle returned is_liked=true, want false")
	}
	persistedLike, err = app.Queries.IsMovieLiked(context.Background(), database.IsMovieLikedParams{
		UserID:  userID,
		MovieID: movieID,
	})
	if err != nil {
		t.Fatalf("IsMovieLiked after unlike: %v", err)
	}
	if persistedLike {
		t.Fatal("expected liked row to be removed after second toggle")
	}
}
