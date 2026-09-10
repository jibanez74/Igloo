package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
		for _, key := range []string{"file_path", "identity_key", "title_key", "artist_key", "artist_tag", "artist_sort", "album_sort", "spotify_date", "source"} {
			exposed := strings.Contains(response.Body.String(), fmt.Sprintf("%q:", key))
			if exposed {
				t.Fatalf("%s exposed internal field %s", operationID, key)
			}
		}
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
	_, err := app.DB.Exec(`
 INSERT INTO music_artist_identity(identity_key,musician_id) VALUES('contract artist',?);
 INSERT INTO music_album_identity(title_key,artist_key,album_id) VALUES('contract album','contract artist',?);
 INSERT INTO music_track_metadata(track_id,artist_tag,artist_key,artist_sort,album_sort) VALUES(?,'Contract Artist','contract artist','Artist, Contract','Album, Contract');
 INSERT INTO music_album_metadata(album_id,spotify_date) VALUES(?,'2000-01-01');
 `, musicianID, albumID, trackID, albumID)
	if err != nil {
		t.Fatal(err)
	}

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

func TestListeningStatisticsPaginationConformsToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()
	user := createTestUser(t, app, "Listener", "listener@example.com", false)
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)
	endpoints := []struct {
		path, operation   string
		defaultLimit, cap int64
		hasOffset         bool
	}{
		{"top-tracks", "getUserTopTracks", 20, 100, true},
		{"top-musicians", "getUserTopMusicians", 10, 50, true},
		{"top-genres", "getUserTopGenres", 10, 20, false},
		{"top-albums", "getUserTopAlbums", 10, 50, true},
		{"recently-played", "getUserRecentlyPlayed", 20, 50, true},
	}
	for _, endpoint := range endpoints {
		for _, query := range []string{"", "limit=", "limit=abc", "limit=1.5", "limit=0", "limit=-1", "limit=9223372036854775808", "limit=1", "limit=999", "offset=7", "offset=-1", "offset=abc", "offset=9223372036854775808"} {
			t.Run(endpoint.path+"/"+query, func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, "/api/music/user-stats/"+endpoint.path+"?"+query, nil)
				request.AddCookie(cookie)
				response := httptest.NewRecorder()
				app.Router.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					t.Fatalf("status = %d: %s", response.Code, response.Body.String())
				}
				var body struct {
					Data struct {
						Limit  int64  `json:"limit"`
						Offset *int64 `json:"offset"`
					} `json:"data"`
				}
				err := json.Unmarshal(response.Body.Bytes(), &body)
				if err != nil {
					t.Fatal(err)
				}
				wantLimit := endpoint.defaultLimit
				if query == "limit=1" {
					wantLimit = 1
				}
				if query == "limit=999" {
					wantLimit = endpoint.cap
				}
				if body.Data.Limit != wantLimit {
					t.Fatalf("limit = %d, want %d", body.Data.Limit, wantLimit)
				}
				if endpoint.hasOffset {
					wantOffset := int64(0)
					if query == "offset=7" {
						wantOffset = 7
					}
					if body.Data.Offset == nil || *body.Data.Offset != wantOffset {
						t.Fatalf("offset = %v, want %d", body.Data.Offset, wantOffset)
					}
				} else if body.Data.Offset != nil {
					t.Fatal("genres unexpectedly returned offset")
				}
				// Lenient parsing accepts values outside the documented integer input type.
				assertOpenAPIResponse(t, endpoint.operation, request, response)
			})
		}
	}
}
