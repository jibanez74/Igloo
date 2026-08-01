package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// newOpenAPIJSONRequest builds a request that a handler and then
// assertOpenAPIExchange can both read. Serving the request drains its body, so
// the assertion replays it through GetBody. Real clients always send the
// content type the contract documents, so set it here too.
func newOpenAPIJSONRequest(method, target, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(body)), nil
	}
	return request
}

// assertOpenAPIExchange validates the observable HTTP boundary rather than a
// handler implementation detail. Call it from endpoint tests after the real
// request has been served. Requests carrying a body must come from
// newOpenAPIJSONRequest so the consumed body can be replayed.
func assertOpenAPIExchange(t *testing.T, operationID string, request *http.Request, response *httptest.ResponseRecorder) {
	t.Helper()

	_, router := loadOpenAPIContract(t)

	route, pathParams, err := router.FindRoute(request)
	if err != nil {
		t.Fatalf("find OpenAPI route for %s %s: %v", request.Method, request.URL.Path, err)
	}
	if route.Operation.OperationID != operationID {
		t.Fatalf("operation ID = %q, want %q", route.Operation.OperationID, operationID)
	}

	if route.Operation.RequestBody != nil && request.GetBody == nil {
		t.Fatalf("operation %s sends a request body; build the request with newOpenAPIJSONRequest so it can be replayed", operationID)
	}
	if request.GetBody != nil {
		replayed, replayErr := request.GetBody()
		if replayErr != nil {
			t.Fatalf("replay request body for %s: %v", operationID, replayErr)
		}
		request.Body = replayed
	}

	requestInput := &openapi3filter.RequestValidationInput{
		Request:    request,
		PathParams: pathParams,
		Route:      route,
		Options:    openAPIValidationOptions,
	}
	err = openapi3filter.ValidateRequest(context.Background(), requestInput)
	if err != nil {
		t.Fatalf("OpenAPI request validation: %v", err)
	}

	result := response.Result()
	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput,
		Status:                 result.StatusCode,
		Header:                 result.Header,
		Body:                   io.NopCloser(bytes.NewBuffer(response.Body.Bytes())),
	}
	err = openapi3filter.ValidateResponse(context.Background(), responseInput)
	if err != nil {
		t.Fatalf("OpenAPI response validation: %v", err)
	}
}

// The contract document is large, so parse, validate, and route-index it once
// for the whole package rather than per assertion.
var (
	openAPIContractOnce   sync.Once
	openAPIContractDoc    *openapi3.T
	openAPIContractRouter routers.Router
	openAPIContractErr    error
)

// openAPIValidationOptions checks that a request documented as authenticated
// actually carries the credential its security scheme names. Whether that
// credential grants access is the middleware's concern and is covered by the
// handler tests; this helper only validates the documented HTTP boundary.
var openAPIValidationOptions = &openapi3filter.Options{
	AuthenticationFunc: assertOpenAPICredentialPresent,
}

func assertOpenAPICredentialPresent(_ context.Context, input *openapi3filter.AuthenticationInput) error {
	scheme := input.SecurityScheme

	isCookieScheme := scheme.Type == "apiKey" && scheme.In == "cookie"
	if isCookieScheme {
		_, err := input.RequestValidationInput.Request.Cookie(scheme.Name)
		if err != nil {
			return fmt.Errorf("security scheme %q requires cookie %q: %w", input.SecuritySchemeName, scheme.Name, err)
		}
		return nil
	}

	isBearerScheme := scheme.Type == "http" && strings.EqualFold(scheme.Scheme, "bearer")
	if isBearerScheme {
		header := input.RequestValidationInput.Request.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return fmt.Errorf("security scheme %q requires a Bearer Authorization header", input.SecuritySchemeName)
		}
		return nil
	}

	return fmt.Errorf("unsupported security scheme %q", input.SecuritySchemeName)
}

func loadOpenAPIContract(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	openAPIContractOnce.Do(loadOpenAPIContractOnce)
	if openAPIContractErr != nil {
		t.Fatalf("load OpenAPI contract: %v", openAPIContractErr)
	}
	return openAPIContractDoc, openAPIContractRouter
}

func loadOpenAPIContractOnce() {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		openAPIContractErr = errors.New("failed to locate OpenAPI contract test")
		return
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))
	documentPath := filepath.Join(repoRoot, "docs", "openapi.json")
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	document, err := loader.LoadFromFile(documentPath)
	if err != nil {
		openAPIContractErr = fmt.Errorf("load OpenAPI document: %w", err)
		return
	}

	err = document.Validate(context.Background())
	if err != nil {
		openAPIContractErr = fmt.Errorf("validate OpenAPI document: %w", err)
		return
	}

	router, err := legacy.NewRouter(document)
	if err != nil {
		openAPIContractErr = fmt.Errorf("create OpenAPI router: %w", err)
		return
	}

	openAPIContractDoc = document
	openAPIContractRouter = router
}

func TestHealthCheckConformsToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	t.Cleanup(func() { _ = app.DB.Close() })
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	app.HealthCheck(response, request)

	assertOpenAPIExchange(t, "healthCheck", request, response)
}

// Every JSON operation needs an explicit endpoint-level assertion. The list
// names the existing focused handler test that owns that exchange; adding an
// operation without adding its assertion makes this test fail.
var openAPIJSONOperationAssertions = map[string]string{
	"addCollaborator":                 "track_playlist_handler_test.go",
	"addMoviePlaylistCollaborator":    "movie_playlist_handler_test.go",
	"addMoviesToMoviePlaylist":        "movie_playlist_handler_test.go",
	"addTracksToPlaylist":             "track_playlist_handler_test.go",
	"adminCreateUser":                 "admin_user_handler_test.go",
	"adminDeleteUser":                 "admin_user_handler_test.go",
	"adminGetUsers":                   "admin_user_handler_test.go",
	"adminResetUserPassword":          "admin_user_handler_test.go",
	"adminUpdateUser":                 "admin_user_handler_test.go",
	"approveQuickConnect":             "quick_connect_handler_test.go",
	"authenticateDevice":              "device_handler_test.go",
	"authenticateUser":                "auth_handler_test.go",
	"createMoviePlaylist":             "movie_playlist_handler_test.go",
	"createNotification":              "notifications_handler_test.go",
	"createPlaylist":                  "playlist_handler_test.go",
	"createWatchRoom":                 "watch_room_handler_test.go",
	"deleteAlbum":                     "stream_file_test.go",
	"deleteMovie":                     "stream_file_test.go",
	"deleteMoviePlaylist":             "movie_playlist_handler_test.go",
	"deleteMovieWatchProgress":        "watch_progress_handler_test.go",
	"deleteNotification":              "notifications_handler_test.go",
	"deletePlaylist":                  "playlist_handler_test.go",
	"deleteUserAccount":               "user_handler_test.go",
	"deleteWatchRoom":                 "watch_room_handler_test.go",
	"destroySession":                  "auth_handler_test.go",
	"getAlbumDetails":                 "album_handler_test.go",
	"getAlbumsAlphabetical":           "album_handler_test.go",
	"getCurrentAuthUser":              "auth_handler_test.go",
	"getContinueWatchingMovies":       "movie_handler_test.go",
	"getDevices":                      "device_handler_test.go",
	"getLikedTracks":                  "track_handler_test.go",
	"getLikedMovies":                  "movie_handler_test.go",
	"getLikedTrackIDsForUser":         "track_handler_test.go",
	"getLatestAlbums":                 "album_handler_test.go",
	"getLatestMovies":                 "movie_handler_test.go",
	"getMovieDetails":                 "movie_handler_test.go",
	"getMovieByTmdbID":                "tmdb_handler_test.go",
	"getMovieGenresList":              "movie_handler_test.go",
	"getMovieLikeStatus":              "movie_like_handler_test.go",
	"getMoviePlaylist":                "movie_playlist_handler_test.go",
	"getMoviePlaylistCollaborators":   "movie_playlist_handler_test.go",
	"getMoviePlaylistMovies":          "movie_playlist_handler_test.go",
	"getMoviePlaylists":               "movie_playlist_handler_test.go",
	"getMovieTechnicalDetails":        "movie_media_test.go",
	"getMovieWatchProgress":           "watch_progress_handler_test.go",
	"getMoviesByGenreLibrary":         "movie_handler_test.go",
	"getMoviesLibrary":                "movie_handler_test.go",
	"getMoviesInTheaters":             "tmdb_handler_test.go",
	"getMoviesStats":                  "stats_handler_test.go",
	"getMusicStats":                   "stats_handler_test.go",
	"getMusicianDetails":              "musician_handler_test.go",
	"getMusiciansAlphabetical":        "musician_handler_test.go",
	"getPlaylist":                     "playlist_handler_test.go",
	"getPlaylistCollaborators":        "playlist_handler_test.go",
	"getPlaylistTracks":               "playlist_handler_test.go",
	"getPlaylists":                    "playlist_handler_test.go",
	"getSettings":                     "settings_handler_test.go",
	"getGeneralSettings":              "settings_handler_test.go",
	"getPlaybackSettings":             "playback_settings_handler_test.go",
	"getShuffleTracks":                "track_handler_test.go",
	"getSpotifyStatus":                "spotify_handler_test.go",
	"getTmdbStatus":                   "tmdb_handler_test.go",
	"getTrackByID":                    "track_handler_test.go",
	"getTracksAlphabetical":           "track_handler_test.go",
	"getUnreadNotificationCount":      "notifications_handler_test.go",
	"getUserListeningStats":           "stats_handler_test.go",
	"getUserRecentlyPlayed":           "stats_handler_test.go",
	"getUserTopAlbums":                "stats_handler_test.go",
	"getUserTopGenres":                "stats_handler_test.go",
	"getUserTopMusicians":             "stats_handler_test.go",
	"getUserTopTracks":                "stats_handler_test.go",
	"getUsers":                        "user_list_handler.go",
	"getUserPin":                      "user_pin_handler_test.go",
	"getWatchRoom":                    "watch_room_handler_test.go",
	"getWatchRooms":                   "watch_room_handler_test.go",
	"healthCheck":                     "openapi_contract_test.go",
	"identifyMovie":                   "movie_edit_handler_test.go",
	"initiateQuickConnect":            "quick_connect_handler_test.go",
	"joinWatchRoom":                   "watch_room_handler_test.go",
	"listNotifications":               "notifications_handler_test.go",
	"lookupQuickConnect":              "quick_connect_handler_test.go",
	"markAllNotificationsRead":        "notifications_handler_test.go",
	"markNotificationRead":            "notifications_handler_test.go",
	"recordPlayEvent":                 "stats_handler_test.go",
	"redeemQuickConnect":              "quick_connect_handler_test.go",
	"revokeDevice":                    "device_handler_test.go",
	"removeCollaborator":              "track_playlist_handler_test.go",
	"removeMovieFromMoviePlaylist":    "movie_playlist_handler_test.go",
	"removeMoviePlaylistCollaborator": "movie_playlist_handler_test.go",
	"removeTrackFromPlaylist":         "track_playlist_handler_test.go",
	"renameDevice":                    "device_handler_test.go",
	"reorderPlaylistTracks":           "track_playlist_handler_test.go",
	"searchAlbums":                    "search_handler_test.go",
	"searchAll":                       "search_handler_test.go",
	"searchMovies":                    "search_handler_test.go",
	"searchMusicians":                 "search_handler_test.go",
	"searchSpotifyAlbums":             "spotify_handler_test.go",
	"searchSpotifyTracks":             "spotify_handler_test.go",
	"searchTmdbMovies":                "tmdb_handler_test.go",
	"searchTracks":                    "search_handler_test.go",
	"setMovieWatched":                 "watch_progress_handler_test.go",
	"stopPersonalHlsSession":          "hls_session_test.go",
	"toggleLikeMovie":                 "movie_like_handler_test.go",
	"toggleLikeTrack":                 "track_handler_test.go",
	"tmdbSearchMovies":                "tmdb_handler_test.go",
	"triggerMovieScan":                "settings_handler_test.go",
	"triggerMusicScan":                "settings_handler_test.go",
	"updateGeneralSettings":           "settings_handler_test.go",
	"updateLibrarySettings":           "settings_handler_test.go",
	"updateMovieMetadata":             "movie_edit_handler_test.go",
	"updateMoviePlaylist":             "movie_playlist_handler_test.go",
	"updateMovieWatchProgress":        "watch_progress_handler_test.go",
	"updatePlaybackSettings":          "playback_settings_handler_test.go",
	"updatePlaylist":                  "playlist_handler_test.go",
	"updateUserAvatar":                "user_handler_test.go",
	"updateUserEmail":                 "user_handler_test.go",
	"updateUserName":                  "user_handler_test.go",
	"updateUserPassword":              "user_handler_test.go",
	"updateUserPin":                   "user_pin_handler_test.go",
	"uploadUserAvatar":                "user_handler_test.go",
	"verifyUserPin":                   "user_pin_handler_test.go",
}

func TestEveryJSONOpenAPIOperationHasConformanceAssertion(t *testing.T) {
	document, _ := loadOpenAPIContract(t)
	missing := make([]string, 0)
	for _, pathItem := range document.Paths.Map() {
		for _, operation := range pathItem.Operations() {
			if !operationReturnsJSON(operation) {
				continue
			}
			if openAPIJSONOperationAssertions[operation.OperationID] == "" {
				missing = append(missing, operation.OperationID)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("JSON OpenAPI operations without an exchange assertion: %s", strings.Join(missing, ", "))
	}
}

func operationReturnsJSON(operation *openapi3.Operation) bool {
	for status, response := range operation.Responses.Map() {
		if strings.HasPrefix(status, "2") && response.Value.Content["application/json"] != nil {
			return true
		}
	}
	return false
}

func TestOpenAPIContractFileExists(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate contract test")
	}
	path := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "docs", "openapi.json")
	_, err := os.Stat(path)
	if err != nil {
		t.Fatalf("OpenAPI document: %v", err)
	}
}
