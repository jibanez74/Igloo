package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestGetMusicianDetails_RejectsBadIDsAndUnknownRows(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	handler := authenticatedRouter(t, app, member.ID)

	tests := []struct {
		name        string
		target      string
		wantStatus  int
		wantMessage string
	}{
		{"non-numeric id", "/api/music/musicians/abc", http.StatusBadRequest, "invalid musician id"},
		{"unknown musician", "/api/music/musicians/999999", http.StatusNotFound, "musician not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.target, nil))
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			var resp helpers.JSONResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil || !resp.Error || resp.Message != tt.wantMessage {
				t.Fatalf("response = %s (%v), want %q", w.Body.String(), err, tt.wantMessage)
			}
		})
	}
}
