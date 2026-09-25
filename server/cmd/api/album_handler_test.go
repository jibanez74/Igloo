package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestAlbumHandlers_RejectBadIDsUnknownRowsAndNonAdmins(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	albumID := createSearchAlbum(t, app, "Kept Album", "Kept Artist")

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
