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
