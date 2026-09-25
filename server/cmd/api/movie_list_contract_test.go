package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"igloo/cmd/internal/database"
)

// Every movie list operation is otherwise only validated against an empty library,
// where the item schema is never exercised. These run with a seeded movie so the
// array elements are validated against the contract.
func TestMovieListHandlers_ConformToOpenAPIWithRows(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "List User", "movie-list-contract@example.com", false)
	movieID := createSearchMovie(t, app, "Contract List Movie", "/movies/contract-list.mkv")

	_, err := app.DB.Exec(
		"UPDATE movies SET certification = ?, year = ?, poster_path = ? WHERE id = ?",
		"PG-13", 2026, "/contract-list.jpg", movieID,
	)
	if err != nil {
		t.Fatalf("seed movie metadata: %v", err)
	}

	genreID := createMovieGenre(t, app, movieID, "Contract Genre")
	playlistID := createMoviePlaylist(t, app, user.ID, movieID)
	likeMovieForUser(t, app, user.ID, movieID)

	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	operations := []struct {
		operationID string
		path        string
		dataKey     string
	}{
		{operationID: "getLatestMovies", path: "/api/movies/latest", dataKey: "movies"},
		{operationID: "getMoviesLibrary", path: "/api/movies/library", dataKey: "movies"},
		{operationID: "getLikedMovies", path: "/api/movies/liked", dataKey: "movies"},
		{operationID: "getMoviesByGenreLibrary", path: "/api/movies/genres/" + strconv.FormatInt(genreID, 10) + "/movies", dataKey: "movies"},
		{operationID: "getMoviePlaylistMovies", path: "/api/movies/playlists/" + strconv.FormatInt(playlistID, 10) + "/movies", dataKey: "movies"},
		{operationID: "getMoviePlaylists", path: "/api/movies/playlists", dataKey: "playlists"},
		{operationID: "getMovieGenresList", path: "/api/movies/genres", dataKey: "genres"},
	}

	for _, operation := range operations {
		t.Run(operation.operationID, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, operation.path, nil)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusOK, response.Body.String())
			}

			assertResponseListNotEmpty(t, operation.operationID, response.Body.Bytes(), operation.dataKey)
			assertOpenAPIExchange(t, operation.operationID, request, response)
		})
	}
}

func createMovieGenre(t *testing.T, app *Application, movieID int64, name string) int64 {
	t.Helper()

	genreID, err := app.Queries.GetOrCreateGenre(t.Context(), database.GetOrCreateGenreParams{
		Tag:       name,
		GenreType: "movie",
	})
	if err != nil {
		t.Fatalf("create genre %q: %v", name, err)
	}
	err = app.Queries.CreateMovieGenre(t.Context(), database.CreateMovieGenreParams{
		MovieID: movieID,
		GenreID: genreID,
	})
	if err != nil {
		t.Fatalf("link genre %q: %v", name, err)
	}
	return genreID
}

func createMoviePlaylist(t *testing.T, app *Application, userID, movieID int64) int64 {
	t.Helper()

	playlist, err := app.Queries.CreateMoviePlaylist(t.Context(), database.CreateMoviePlaylistParams{
		UserID:   userID,
		Name:     "Contract Playlist",
		IsPublic: false,
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}
	_, err = app.DB.Exec(
		"INSERT INTO playlist_movies (playlist_id, movie_id, position, added_by) VALUES (?, ?, 1, ?)",
		playlist.ID, movieID, userID,
	)
	if err != nil {
		t.Fatalf("add movie to playlist: %v", err)
	}
	return playlist.ID
}

func likeMovieForUser(t *testing.T, app *Application, userID, movieID int64) {
	t.Helper()

	_, err := app.DB.Exec(
		"INSERT INTO user_liked_movies (user_id, movie_id) VALUES (?, ?)",
		userID, movieID,
	)
	if err != nil {
		t.Fatalf("like movie: %v", err)
	}
}

func seedWatchProgress(t *testing.T, app *Application, userID, movieID int64) {
	t.Helper()

	_, err := app.DB.Exec(
		fmt.Sprintf("INSERT INTO movie_watch_progress (user_id, movie_id, progress_sec, duration_sec) VALUES (?, ?, %d, %d)", 600, 5400),
		userID, movieID,
	)
	if err != nil {
		t.Fatalf("seed watch progress: %v", err)
	}
}
