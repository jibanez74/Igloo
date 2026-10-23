package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func TestAlbumHandlers_RejectBadIDsUnknownRowsAndNonAdmins(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	albumID := createTestAlbum(t, app, "Kept Album", "Kept Artist")

	tests := []struct {
		name        string
		userID      int64
		method      string
		target      string
		wantStatus  int
		wantMessage string
	}{
		{"details with a non-numeric id", member.ID, http.MethodGet, "/api/music/albums/details/abc", http.StatusBadRequest, "invalid album id"},
		{"details for an unknown album", member.ID, http.MethodGet, "/api/music/albums/details/999999", http.StatusNotFound, "album not found"},
		{"delete by a member", member.ID, http.MethodDelete, "/api/music/albums/1", http.StatusForbidden, ""},
		{"delete with a non-numeric id", admin.ID, http.MethodDelete, "/api/music/albums/abc", http.StatusBadRequest, "invalid album id"},
		{"delete an unknown album", admin.ID, http.MethodDelete, "/api/music/albums/999999", http.StatusNotFound, "album not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			authenticatedRouter(t, app, tt.userID).ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, nil))
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantMessage == "" {
				return
			}
			var resp helpers.JSONResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil || !resp.Error || resp.Message != tt.wantMessage {
				t.Fatalf("response = %s (%v), want %q", w.Body.String(), err, tt.wantMessage)
			}
		})
	}

	// None of the rejections touched the existing album.
	_, err := app.Queries.GetAlbumByID(t.Context(), albumID)
	if err != nil {
		t.Fatalf("kept album lookup: %v", err)
	}
}

// spotify_url is derived from the stored id, and album_type is whatever the
// Spotify match reported; both are null for an unmatched album.
func TestGetAlbumDetails_DerivesSpotifyURLAndReportsAlbumType(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	handler := authenticatedRouter(t, app, member.ID)
	ctx := context.Background()

	matched, err := app.Queries.UpsertAlbum(ctx, database.UpsertAlbumParams{
		Title:     "Matched Album",
		SortTitle: "Matched Album",
		Musician:  sql.NullString{String: "Matched Artist", Valid: true},
		SpotifyID: sql.NullString{String: "6dVIqQ8qmQ5GBnJ9shOYGE", Valid: true},
	})
	if err != nil {
		t.Fatalf("create matched album: %v", err)
	}
	err = app.Queries.UpdateMusicAlbumEnrichment(ctx, database.UpdateMusicAlbumEnrichmentParams{ID: matched.ID, AlbumType: sql.NullString{String: "single", Valid: true}})
	if err != nil {
		t.Fatalf("set album type: %v", err)
	}
	unmatchedID := createTestAlbum(t, app, "Local Album", "Local Artist")

	tests := []struct {
		name          string
		albumID       int64
		wantURL       any
		wantAlbumType map[string]any
	}{
		{"matched album", matched.ID, "https://open.spotify.com/album/6dVIqQ8qmQ5GBnJ9shOYGE", map[string]any{"String": "single", "Valid": true}},
		{"unmatched album", unmatchedID, nil, map[string]any{"String": "", "Valid": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/music/albums/details/%d", tt.albumID), nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
			var resp struct {
				Data struct {
					Album map[string]any `json:"album"`
				} `json:"data"`
			}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("decode response: %v", err)
			}
			got, present := resp.Data.Album["spotify_url"]
			if !present || got != tt.wantURL {
				t.Fatalf("spotify_url = %v (present %v), want %v", got, present, tt.wantURL)
			}
			if !reflect.DeepEqual(resp.Data.Album["album_type"], tt.wantAlbumType) {
				t.Fatalf("album_type = %v, want %v", resp.Data.Album["album_type"], tt.wantAlbumType)
			}
		})
	}
}
