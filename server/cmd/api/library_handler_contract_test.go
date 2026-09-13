package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

func TestLibraryAndStatisticsHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Contract User", "contract-user@example.com", false)

	app.InitSession()
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)

	assertRequest := func(operationID string, req *http.Request, wantStatus int, dataKeys ...string) {
		t.Helper()
		req.AddCookie(cookie)
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, req)
		if response.Code != wantStatus {
			t.Fatalf("%s status = %d, want %d, body = %s", operationID, response.Code, wantStatus, response.Body.String())
		}
		assertResponseListNotEmpty(t, operationID, response.Body.Bytes(), dataKeys...)
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
		{operationID: "getLatestShows", path: "/api/shows/latest"},
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
	seedMovieDetailFixtures(t, app, movieID)
	musicianID := createSearchMusician(t, app, "Contract Artist")
	albumID := createSearchAlbum(t, app, "Contract Album", "Contract Artist")
	trackID := createSearchTrack(t, app, "Contract Track", "/music/contract.flac", albumID, musicianID)
	seedTrackRelationships(t, app, trackID, musicianID, "Contract Track Genre")
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
		dataKeys    []string
	}{
		{
			operationID: "getAlbumDetails",
			path:        "/api/music/albums/details/" + albumIDString,
			dataKeys:    []string{"tracks", "artists", "track_genres", "album_genres"},
		},
		{
			operationID: "getMusicianDetails",
			path:        "/api/music/musicians/" + musicianIDString,
			dataKeys:    []string{"albums", "tracks", "genres"},
		},
		{operationID: "getTrackByID", path: "/api/music/tracks/details/" + trackIDString},
		{
			operationID: "getMovieDetails",
			path:        "/api/movies/details/" + movieIDString,
			dataKeys:    []string{"cast", "crew", "genres", "production_companies", "extra_videos"},
		},
		{
			operationID: "getMovieTechnicalDetails",
			path:        "/api/movies/" + movieIDString + "/technical-details",
			dataKeys:    []string{"video_streams", "audio_streams", "subtitles", "chapters"},
		},
	}
	for _, operation := range detailOperations {
		t.Run(operation.operationID, func(t *testing.T) {
			assertRequest(operation.operationID, httptest.NewRequest(http.MethodGet, operation.path, nil), http.StatusOK, operation.dataKeys...)
		})
	}

	assertRequest("toggleLikeTrack", httptest.NewRequest(http.MethodPost, "/api/music/tracks/"+trackIDString+"/like", nil), http.StatusOK)
	assertRequest("toggleLikeMovie", httptest.NewRequest(http.MethodPost, "/api/movies/"+movieIDString+"/like", nil), http.StatusOK)

	playBody := fmt.Sprintf(`{"track_id":%d,"duration_played":120,"completed":true}`, trackID)
	assertRequest("recordPlayEvent", newOpenAPIJSONRequest(http.MethodPost, "/api/music/user-stats/play", playBody), http.StatusOK)

	// The listening statistics above ran against an empty play history. Repeat
	// them now that one play event exists, so the item schemas are validated.
	playedOperations := []struct {
		operationID string
		path        string
		dataKey     string
	}{
		{operationID: "getUserTopTracks", path: "/api/music/user-stats/top-tracks", dataKey: "tracks"},
		{operationID: "getUserTopMusicians", path: "/api/music/user-stats/top-musicians", dataKey: "musicians"},
		{operationID: "getUserTopGenres", path: "/api/music/user-stats/top-genres", dataKey: "genres"},
		{operationID: "getUserTopAlbums", path: "/api/music/user-stats/top-albums", dataKey: "albums"},
		{operationID: "getUserRecentlyPlayed", path: "/api/music/user-stats/recently-played", dataKey: "tracks"},
	}
	for _, operation := range playedOperations {
		t.Run(operation.operationID+"/played", func(t *testing.T) {
			assertRequest(operation.operationID, httptest.NewRequest(http.MethodGet, operation.path, nil), http.StatusOK, operation.dataKey)
		})
	}
}

// getMovieDetails and getMovieTechnicalDetails return six arrays that a bare
// movie row leaves empty, which would leave their item schemas unvalidated.
func seedMovieDetailFixtures(t *testing.T, app *Application, movieID int64) {
	t.Helper()

	ctx := t.Context()

	artistID, err := app.Queries.UpsertArtist(ctx, database.UpsertArtistParams{
		Name:    "Contract Artist Credit",
		TmdbID:  9001,
		Profile: sql.NullString{String: "/artist.jpg", Valid: true},
	})
	if err != nil {
		t.Fatalf("create artist: %v", err)
	}
	err = app.Queries.UpsertCast(ctx, database.UpsertCastParams{
		MovieID:   movieID,
		ArtistID:  artistID,
		Character: "Contract Character",
		CastOrder: 1,
	})
	if err != nil {
		t.Fatalf("create cast credit: %v", err)
	}
	err = app.Queries.UpsertCrew(ctx, database.UpsertCrewParams{
		MovieID:    movieID,
		ArtistID:   artistID,
		Job:        "Director",
		Department: "Directing",
	})
	if err != nil {
		t.Fatalf("create crew credit: %v", err)
	}

	createMovieGenre(t, app, movieID, "Contract Movie Genre")

	companyID, err := app.Queries.UpsertProductionCompany(ctx, database.UpsertProductionCompanyParams{
		Name:   "Contract Studio",
		TmdbID: 9002,
	})
	if err != nil {
		t.Fatalf("create production company: %v", err)
	}
	err = app.Queries.CreateMovieProductionCompany(ctx, database.CreateMovieProductionCompanyParams{
		MovieID:             movieID,
		ProductionCompanyID: companyID,
	})
	if err != nil {
		t.Fatalf("link production company: %v", err)
	}

	extraVideoID, err := app.Queries.UpsertExtraVideo(ctx, database.UpsertExtraVideoParams{
		Title:      "Contract Trailer",
		ExternalID: sql.NullString{String: "contract-trailer", Valid: true},
		Key:        "contract-key",
		Type:       "trailer",
		Site:       "youtube",
	})
	if err != nil {
		t.Fatalf("create extra video: %v", err)
	}
	err = app.Queries.CreateMovieExtraVideo(ctx, database.CreateMovieExtraVideoParams{
		MovieID:      movieID,
		ExtraVideoID: extraVideoID,
	})
	if err != nil {
		t.Fatalf("link extra video: %v", err)
	}

	err = app.Queries.InsertVideoStream(ctx, database.InsertVideoStreamParams{
		MovieID:     movieID,
		StreamIndex: 0,
		Codec:       "h264",
		BitRate:     5_000_000,
		Width:       1920,
		Height:      1080,
		FrameRate:   23.976,
	})
	if err != nil {
		t.Fatalf("create video stream: %v", err)
	}
	err = app.Queries.InsertAudioStream(ctx, database.InsertAudioStreamParams{
		MovieID:     movieID,
		StreamIndex: 1,
		Codec:       "aac",
		BitRate:     192_000,
		Channels:    2,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("create audio stream: %v", err)
	}
	err = app.Queries.InsertSubtitle(ctx, database.InsertSubtitleParams{
		MovieID:     movieID,
		StreamIndex: 2,
		Codec:       "subrip",
		Language:    sql.NullString{String: "eng", Valid: true},
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("create subtitle: %v", err)
	}
	err = app.Queries.InsertChapter(ctx, database.InsertChapterParams{
		MovieID:   movieID,
		Title:     "Contract Chapter",
		StartTime: 0,
	})
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
}

// A track's album artists, its musician's albums and its musician's genres are
// all reconciled by triggers on track_musicians, and the genre trigger copies
// whatever track genres exist at that moment, so the credit is inserted last.
func seedTrackRelationships(t *testing.T, app *Application, trackID, musicianID int64, tag string) {
	t.Helper()

	ctx := t.Context()

	genreID, err := app.Queries.GetOrCreateGenre(ctx, database.GetOrCreateGenreParams{
		Tag:       tag,
		GenreType: "music",
	})
	if err != nil {
		t.Fatalf("create genre %q: %v", tag, err)
	}
	err = app.Queries.CreateTrackGenre(ctx, database.CreateTrackGenreParams{
		TrackID: trackID,
		GenreID: genreID,
	})
	if err != nil {
		t.Fatalf("link genre %q: %v", tag, err)
	}

	err = app.Queries.CreateTrackMusician(ctx, database.CreateTrackMusicianParams{
		TrackID:    trackID,
		MusicianID: musicianID,
	})
	if err != nil {
		t.Fatalf("credit musician on track: %v", err)
	}
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

func TestRecordPlayEventDurationBoundaries(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()
	user := createTestUser(t, app, "Listener", "duration@example.com", false)
	musicianID := createSearchMusician(t, app, "Artist")
	albumID := createSearchAlbum(t, app, "Album", "Artist")
	trackID := createSearchTrack(t, app, "Track", "/music/duration.flac", albumID, musicianID)
	app.InitRouter()
	cookie := newAuthSessionCookie(t, app, user.ID)
	for _, tc := range []struct {
		name              string
		trackID, duration int64
		status            int
	}{
		{"negative duration", trackID, -1, http.StatusBadRequest},
		{"negative track", -1, 0, http.StatusBadRequest},
		{"zero duration", trackID, 0, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"track_id":%d,"duration_played":%d,"completed":false}`, tc.trackID, tc.duration)
			request := newOpenAPIJSONRequest(http.MethodPost, "/api/music/user-stats/play", body)
			request.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status=%d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
			if tc.status == http.StatusOK {
				assertOpenAPIExchange(t, "recordPlayEvent", request, response)
			} else {
				assertOpenAPIResponse(t, "recordPlayEvent", request, response)
				var count int
				err := app.DB.QueryRow("SELECT count(*) FROM user_play_history").Scan(&count)
				if err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("rejected play event was stored")
				}
			}
		})
	}
}
