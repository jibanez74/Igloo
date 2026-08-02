package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestLibraryAndStatisticsHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Contract User", "contract-user@example.com", false)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	assertRequest := func(operationID string, req *http.Request, wantStatus int) {
		t.Helper()
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, req)
		if response.Code != wantStatus {
			t.Fatalf("%s status = %d, want %d, body = %s", operationID, response.Code, wantStatus, response.Body.String())
		}
		assertOpenAPIExchange(t, operationID, req, response)
	}

	emptyGetOperations := []struct {
		operationID string
		path        string
	}{
		{operationID: "getAlbumsAlphabetical", path: "/api/music/albums"},
		{operationID: "getLatestAlbums", path: "/api/music/albums/latest"},
		{operationID: "getMusiciansAlphabetical", path: "/api/music/musicians"},
		{operationID: "getTracksAlphabetical", path: "/api/music/tracks"},
		{operationID: "getShuffleTracks", path: "/api/music/tracks/shuffle"},
		{operationID: "getMusicStats", path: "/api/music/stats"},
		{operationID: "getLatestMovies", path: "/api/movies/latest"},
		{operationID: "getMoviesLibrary", path: "/api/movies/library"},
		{operationID: "getMoviesStats", path: "/api/movies/stats"},
		{operationID: "getMovieGenresList", path: "/api/movies/genres"},
		{operationID: "getMoviesByGenreLibrary", path: "/api/movies/genres/1/movies"},
		{operationID: "getLikedTracks", path: "/api/music/tracks/liked"},
		{operationID: "getLikedTrackIDsForUser", path: "/api/music/tracks/liked-ids"},
		{operationID: "getLikedMovies", path: "/api/movies/liked"},
		{operationID: "getUserListeningStats", path: "/api/music/user-stats/overview"},
		{operationID: "getUserTopTracks", path: "/api/music/user-stats/top-tracks"},
		{operationID: "getUserTopMusicians", path: "/api/music/user-stats/top-musicians"},
		{operationID: "getUserTopGenres", path: "/api/music/user-stats/top-genres"},
		{operationID: "getUserTopAlbums", path: "/api/music/user-stats/top-albums"},
		{operationID: "getUserRecentlyPlayed", path: "/api/music/user-stats/recently-played"},
	}
	for _, operation := range emptyGetOperations {
		t.Run(operation.operationID, func(t *testing.T) {
			assertRequest(operation.operationID, httptest.NewRequest(http.MethodGet, operation.path, nil), http.StatusOK)
		})
	}

	movieID := createSearchMovie(t, app, "Contract Movie", "/movies/contract.mkv")
	musicianID := createSearchMusician(t, app, "Contract Artist")
	albumID := createSearchAlbum(t, app, "Contract Album", "Contract Artist")
	trackID := createSearchTrack(t, app, "Contract Track", "/music/contract.flac", albumID, musicianID)
	movieIDString := strconv.FormatInt(movieID, 10)
	albumIDString := strconv.FormatInt(albumID, 10)
	musicianIDString := strconv.FormatInt(musicianID, 10)
	trackIDString := strconv.FormatInt(trackID, 10)

	detailOperations := []struct {
		operationID string
		path        string
	}{
		{operationID: "getAlbumDetails", path: "/api/music/albums/details/" + albumIDString},
		{operationID: "getMusicianDetails", path: "/api/music/musicians/" + musicianIDString},
		{operationID: "getTrackByID", path: "/api/music/tracks/details/" + trackIDString},
		{operationID: "getMovieDetails", path: "/api/movies/details/" + movieIDString},
		{operationID: "getMovieTechnicalDetails", path: "/api/movies/" + movieIDString + "/technical-details"},
	}
	for _, operation := range detailOperations {
		t.Run(operation.operationID, func(t *testing.T) {
			assertRequest(operation.operationID, httptest.NewRequest(http.MethodGet, operation.path, nil), http.StatusOK)
		})
	}

	assertRequest("toggleLikeTrack", httptest.NewRequest(http.MethodPost, "/api/music/tracks/"+trackIDString+"/like", nil), http.StatusOK)
	assertRequest("toggleLikeMovie", httptest.NewRequest(http.MethodPost, "/api/movies/"+movieIDString+"/like", nil), http.StatusOK)

	playBody := fmt.Sprintf(`{"track_id":%d,"duration_played":120,"completed":true}`, trackID)
	assertRequest("recordPlayEvent", newOpenAPIJSONRequest(http.MethodPost, "/api/music/user-stats/play", playBody), http.StatusOK)
}
