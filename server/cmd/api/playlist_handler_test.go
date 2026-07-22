package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"

	"github.com/go-chi/chi/v5"
)

func TestValidatePlaylistMetadataCountsUnicodeCodePoints(t *testing.T) {
	tests := []struct {
		name         string
		playlistName string
		description  string
		wantError    string
	}{
		{
			name:      "empty name",
			wantError: "playlist name is required",
		},
		{
			name:         "ASCII name at limit",
			playlistName: strings.Repeat("a", playlistNameMaxLength),
		},
		{
			name:         "ASCII name over limit",
			playlistName: strings.Repeat("a", playlistNameMaxLength+1),
			wantError:    "playlist name is too long (max 255 characters)",
		},
		{
			name:         "multibyte name at limit",
			playlistName: strings.Repeat("😀", playlistNameMaxLength),
		},
		{
			name:         "multibyte name over limit",
			playlistName: strings.Repeat("😀", playlistNameMaxLength+1),
			wantError:    "playlist name is too long (max 255 characters)",
		},
		{
			name:         "ASCII description at limit",
			playlistName: "Playlist",
			description:  strings.Repeat("a", playlistDescriptionMaxLength),
		},
		{
			name:         "ASCII description over limit",
			playlistName: "Playlist",
			description:  strings.Repeat("a", playlistDescriptionMaxLength+1),
			wantError:    "description is too long (max 1000 characters)",
		},
		{
			name:         "multibyte description at limit",
			playlistName: "Playlist",
			description:  strings.Repeat("界", playlistDescriptionMaxLength),
		},
		{
			name:         "multibyte description over limit",
			playlistName: "Playlist",
			description:  strings.Repeat("界", playlistDescriptionMaxLength+1),
			wantError:    "description is too long (max 1000 characters)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePlaylistMetadata(tt.playlistName, tt.description)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validatePlaylistMetadata returned error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validatePlaylistMetadata returned nil error, want %q", tt.wantError)
			}
			if err.Error() != tt.wantError {
				t.Fatalf("validatePlaylistMetadata error = %q, want %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestPlaylistMutationHandlersCountUnicodeCodePoints(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     "Playlist Tester",
		Email:    "playlist-tester@example.com",
		Password: "hashed",
		IsAdmin:  false,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create playlist test user: %v", err)
	}

	trackPlaylist, err := app.Queries.CreatePlaylist(context.Background(), database.CreatePlaylistParams{
		UserID:      user.ID,
		Name:        "Track Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
	})
	if err != nil {
		t.Fatalf("create track playlist: %v", err)
	}

	moviePlaylist, err := app.Queries.CreateMoviePlaylist(context.Background(), database.CreateMoviePlaylistParams{
		UserID:      user.ID,
		Name:        "Movie Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
		MovieID:     sql.NullInt64{},
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}

	authenticated := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), cookieUserID, user.ID)
			next(w, r)
		}
	}

	router := chi.NewRouter()
	router.Post("/api/music/playlists/", authenticated(app.CreatePlaylist))
	router.Put("/api/music/playlists/{id}", authenticated(app.UpdatePlaylist))
	router.Post("/api/movies/playlists/", authenticated(app.CreateMoviePlaylist))
	router.Put("/api/movies/playlists/{id}", authenticated(app.UpdateMoviePlaylist))
	handler := app.SessionManager.LoadAndSave(router)

	validUnicodeName := strings.Repeat("😀", playlistNameMaxLength)
	overLimitUnicodeName := strings.Repeat("😀", playlistNameMaxLength+1)
	tests := []struct {
		name       string
		method     string
		path       string
		value      string
		wantStatus int
	}{
		{
			name:       "create music playlist accepts Unicode at limit",
			method:     http.MethodPost,
			path:       "/api/music/playlists/",
			value:      validUnicodeName,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "create music playlist rejects Unicode over limit",
			method:     http.MethodPost,
			path:       "/api/music/playlists/",
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "update music playlist accepts Unicode at limit",
			method:     http.MethodPut,
			path:       "/api/music/playlists/" + strconv.FormatInt(trackPlaylist.ID, 10),
			value:      validUnicodeName,
			wantStatus: http.StatusOK,
		},
		{
			name:       "update music playlist rejects Unicode over limit",
			method:     http.MethodPut,
			path:       "/api/music/playlists/" + strconv.FormatInt(trackPlaylist.ID, 10),
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "create movie playlist accepts Unicode at limit",
			method:     http.MethodPost,
			path:       "/api/movies/playlists/",
			value:      validUnicodeName,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "create movie playlist rejects Unicode over limit",
			method:     http.MethodPost,
			path:       "/api/movies/playlists/",
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "update movie playlist accepts Unicode at limit",
			method:     http.MethodPut,
			path:       "/api/movies/playlists/" + strconv.FormatInt(moviePlaylist.ID, 10),
			value:      validUnicodeName,
			wantStatus: http.StatusOK,
		},
		{
			name:       "update movie playlist rejects Unicode over limit",
			method:     http.MethodPut,
			path:       "/api/movies/playlists/" + strconv.FormatInt(moviePlaylist.ID, 10),
			value:      overLimitUnicodeName,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]string{
				"name":        tt.value,
				"description": "Description",
			})
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func playlistTestHandler(app *Application) http.Handler {
	app.InitSession()

	router := chi.NewRouter()
	router.Route("/api/music/playlists", func(r chi.Router) {
		r.Get("/", app.GetPlaylists)
		r.Post("/", app.CreatePlaylist)
		r.Get("/{id}", app.GetPlaylist)
		r.Put("/{id}", app.UpdatePlaylist)
		r.Delete("/{id}", app.DeletePlaylist)
		r.Get("/{id}/tracks", app.GetPlaylistTracks)
		r.Post("/{id}/tracks", app.AddTracksToPlaylist)
		r.Delete("/{id}/tracks/{trackId}", app.RemoveTrackFromPlaylist)
		r.Put("/{id}/tracks/reorder", app.ReorderPlaylistTracks)
		r.Get("/{id}/collaborators", app.GetPlaylistCollaborators)
		r.Post("/{id}/collaborators", app.AddCollaborator)
		r.Delete("/{id}/collaborators/{userId}", app.RemoveCollaborator)
	})
	router.Route("/api/movies/playlists", func(r chi.Router) {
		r.Get("/", app.GetMoviePlaylists)
		r.Post("/", app.CreateMoviePlaylist)
		r.Get("/{id}", app.GetMoviePlaylist)
		r.Put("/{id}", app.UpdateMoviePlaylist)
		r.Delete("/{id}", app.DeleteMoviePlaylist)
		r.Get("/{id}/movies", app.GetMoviePlaylistMovies)
		r.Post("/{id}/movies", app.AddMoviesToMoviePlaylist)
		r.Delete("/{id}/movies/{movieId}", app.RemoveMovieFromMoviePlaylist)
	})

	return app.SessionManager.LoadAndSave(router)
}

func playlistAuthCookies(t *testing.T, app *Application, userID int64) []*http.Cookie {
	t.Helper()

	handler := app.SessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/session", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	return resp.Cookies()
}

func performPlaylistRequest(
	t *testing.T,
	app *Application,
	handler http.Handler,
	userID int64,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != 0 {
		for _, cookie := range playlistAuthCookies(t, app, userID) {
			req.AddCookie(cookie)
		}
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

type playlistFixtures struct {
	owner         database.User
	editor        database.User
	viewer        database.User
	outsider      database.User
	trackPlaylist database.Playlist
	moviePlaylist database.Playlist
	trackID       int64
	movieID       int64
}

func createPlaylistFixtures(t *testing.T, app *Application) playlistFixtures {
	t.Helper()

	owner := createTestUser(t, app, "Playlist Owner", "playlist-owner@example.com", false)
	editor := createTestUser(t, app, "Playlist Editor", "playlist-editor@example.com", false)
	viewer := createTestUser(t, app, "Playlist Viewer", "playlist-viewer@example.com", false)
	outsider := createTestUser(t, app, "Playlist Outsider", "playlist-outsider@example.com", false)

	trackPlaylist, err := app.Queries.CreatePlaylist(context.Background(), database.CreatePlaylistParams{
		UserID:      owner.ID,
		Name:        "Track Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
	})
	if err != nil {
		t.Fatalf("create track playlist: %v", err)
	}
	moviePlaylist, err := app.Queries.CreateMoviePlaylist(context.Background(), database.CreateMoviePlaylistParams{
		UserID:      owner.ID,
		Name:        "Movie Playlist",
		Description: sql.NullString{},
		CoverImage:  sql.NullString{},
		IsPublic:    false,
		MovieID:     sql.NullInt64{},
	})
	if err != nil {
		t.Fatalf("create movie playlist: %v", err)
	}

	for _, playlistID := range []int64{trackPlaylist.ID, moviePlaylist.ID} {
		_, err = app.Queries.AddCollaborator(context.Background(), database.AddCollaboratorParams{
			PlaylistID: playlistID,
			UserID:     editor.ID,
			CanEdit:    true,
		})
		if err != nil {
			t.Fatalf("add editor to playlist %d: %v", playlistID, err)
		}
		_, err = app.Queries.AddCollaborator(context.Background(), database.AddCollaboratorParams{
			PlaylistID: playlistID,
			UserID:     viewer.ID,
			CanEdit:    false,
		})
		if err != nil {
			t.Fatalf("add viewer to playlist %d: %v", playlistID, err)
		}
	}

	musicianID := createSearchMusician(t, app, "Playlist Artist")
	albumID := createSearchAlbum(t, app, "Playlist Album", "Playlist Artist")
	trackID := createSearchTrack(t, app, "Playlist Track", "/music/playlist-track.flac", albumID, musicianID)
	movieID := createSearchMovie(t, app, "Playlist Movie", "/movies/playlist-movie.mkv")

	return playlistFixtures{
		owner:         owner,
		editor:        editor,
		viewer:        viewer,
		outsider:      outsider,
		trackPlaylist: trackPlaylist,
		moviePlaylist: moviePlaylist,
		trackID:       trackID,
		movieID:       movieID,
	}
}

func TestPlaylistAccessAndContentTypes(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	handler := playlistTestHandler(app)
	fixtures := createPlaylistFixtures(t, app)

	type detailEnvelope struct {
		Data struct {
			IsOwner bool `json:"is_owner"`
			CanEdit bool `json:"can_edit"`
		} `json:"data"`
	}

	tests := []struct {
		name      string
		userID    int64
		path      string
		wantCode  int
		wantOwner bool
		wantEdit  bool
	}{
		{name: "owner has full track access", userID: fixtures.owner.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusOK, wantOwner: true, wantEdit: true},
		{name: "editor can edit track playlist", userID: fixtures.editor.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusOK, wantEdit: true},
		{name: "viewer can view track playlist", userID: fixtures.viewer.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusOK},
		{name: "outsider cannot view private track playlist", userID: fixtures.outsider.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusForbidden},
		{name: "track endpoint rejects movie playlist", userID: fixtures.owner.ID, path: "/api/music/playlists/" + strconv.FormatInt(fixtures.moviePlaylist.ID, 10), wantCode: http.StatusBadRequest},
		{name: "movie endpoint rejects track playlist", userID: fixtures.owner.ID, path: "/api/movies/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusBadRequest},
		{name: "unauthenticated request is rejected", path: "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10), wantCode: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performPlaylistRequest(t, app, handler, tt.userID, http.MethodGet, tt.path, "")
			if w.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantCode, w.Body.String())
			}
			if tt.wantCode != http.StatusOK {
				return
			}

			var response detailEnvelope
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("decode detail response: %v", err)
			}
			if response.Data.IsOwner != tt.wantOwner || response.Data.CanEdit != tt.wantEdit {
				t.Fatalf("access = owner:%v edit:%v, want owner:%v edit:%v", response.Data.IsOwner, response.Data.CanEdit, tt.wantOwner, tt.wantEdit)
			}
		})
	}
}

func TestPlaylistListsExposeOwnerAndEditorAccess(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	handler := playlistTestHandler(app)
	fixtures := createPlaylistFixtures(t, app)

	type playlistAccess struct {
		ID      int64 `json:"id"`
		IsOwner bool  `json:"is_owner"`
		CanEdit bool  `json:"can_edit"`
	}
	type listEnvelope struct {
		Data struct {
			Playlists []playlistAccess `json:"playlists"`
		} `json:"data"`
	}

	tests := []struct {
		name    string
		userID  int64
		path    string
		wantID  int64
		isOwner bool
		canEdit bool
	}{
		{name: "track owner", userID: fixtures.owner.ID, path: "/api/music/playlists/", wantID: fixtures.trackPlaylist.ID, isOwner: true, canEdit: true},
		{name: "track editor", userID: fixtures.editor.ID, path: "/api/music/playlists/", wantID: fixtures.trackPlaylist.ID, canEdit: true},
		{name: "track viewer", userID: fixtures.viewer.ID, path: "/api/music/playlists/", wantID: fixtures.trackPlaylist.ID},
		{name: "movie owner", userID: fixtures.owner.ID, path: "/api/movies/playlists/", wantID: fixtures.moviePlaylist.ID, isOwner: true, canEdit: true},
		{name: "movie editor", userID: fixtures.editor.ID, path: "/api/movies/playlists/", wantID: fixtures.moviePlaylist.ID, canEdit: true},
		{name: "movie viewer", userID: fixtures.viewer.ID, path: "/api/movies/playlists/", wantID: fixtures.moviePlaylist.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := performPlaylistRequest(t, app, handler, tt.userID, http.MethodGet, tt.path, "")
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}

			var response listEnvelope
			err := json.Unmarshal(w.Body.Bytes(), &response)
			if err != nil {
				t.Fatalf("decode list response: %v", err)
			}
			if len(response.Data.Playlists) != 1 {
				t.Fatalf("playlist count = %d, want 1", len(response.Data.Playlists))
			}
			playlist := response.Data.Playlists[0]
			if playlist.ID != tt.wantID || playlist.IsOwner != tt.isOwner || playlist.CanEdit != tt.canEdit {
				t.Fatalf("playlist access = %#v, want id=%d owner=%v edit=%v", playlist, tt.wantID, tt.isOwner, tt.canEdit)
			}
		})
	}
}

func TestPlaylistEditorsCanMutateContentButViewersCannot(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	handler := playlistTestHandler(app)
	fixtures := createPlaylistFixtures(t, app)

	trackPath := "/api/music/playlists/" + strconv.FormatInt(fixtures.trackPlaylist.ID, 10) + "/tracks"
	moviePath := "/api/movies/playlists/" + strconv.FormatInt(fixtures.moviePlaylist.ID, 10) + "/movies"

	w := performPlaylistRequest(t, app, handler, fixtures.viewer.ID, http.MethodPost, trackPath, `{"track_ids":[`+strconv.FormatInt(fixtures.trackID, 10)+`]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer add track status = %d, want 403: %s", w.Code, w.Body.String())
	}
	w = performPlaylistRequest(t, app, handler, fixtures.editor.ID, http.MethodPost, trackPath, `{"track_ids":[`+strconv.FormatInt(fixtures.trackID, 10)+`]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("editor add track status = %d, want 200: %s", w.Code, w.Body.String())
	}
	w = performPlaylistRequest(t, app, handler, fixtures.editor.ID, http.MethodDelete, trackPath+"/"+strconv.FormatInt(fixtures.trackID, 10), "")
	if w.Code != http.StatusOK {
		t.Fatalf("editor remove track status = %d, want 200: %s", w.Code, w.Body.String())
	}

	w = performPlaylistRequest(t, app, handler, fixtures.viewer.ID, http.MethodPost, moviePath, `{"movie_ids":[`+strconv.FormatInt(fixtures.movieID, 10)+`]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer add movie status = %d, want 403: %s", w.Code, w.Body.String())
	}
	w = performPlaylistRequest(t, app, handler, fixtures.editor.ID, http.MethodPost, moviePath, `{"movie_ids":[`+strconv.FormatInt(fixtures.movieID, 10)+`]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("editor add movie status = %d, want 200: %s", w.Code, w.Body.String())
	}
	w = performPlaylistRequest(t, app, handler, fixtures.editor.ID, http.MethodDelete, moviePath+"/"+strconv.FormatInt(fixtures.movieID, 10), "")
	if w.Code != http.StatusOK {
		t.Fatalf("editor remove movie status = %d, want 200: %s", w.Code, w.Body.String())
	}
}
