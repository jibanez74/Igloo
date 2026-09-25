package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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
