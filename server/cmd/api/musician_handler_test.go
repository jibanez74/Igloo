package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/database"
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
		{"zero id", "/api/music/musicians/0", http.StatusBadRequest, "invalid musician id"},
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

// sort_name is not unique, so the list breaks ties by id; otherwise a page
// boundary between two musicians sharing a sort name could repeat or skip one.
func TestGetMusiciansAlphabetical_BreaksSortNameTiesByID(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	handler := authenticatedRouter(t, app, member.ID)

	var ids []int64
	for _, name := range []string{"The Band", "Band"} {
		musician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
			Name:     name,
			SortName: "band",
		})
		if err != nil {
			t.Fatalf("create musician %q: %v", name, err)
		}
		ids = append(ids, musician.ID)
	}

	for page, wantID := range ids {
		w := httptest.NewRecorder()
		target := fmt.Sprintf("/api/music/musicians?per_page=1&page=%d", page+1)
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("page %d status = %d: %s", page+1, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				Musicians []struct {
					ID int64 `json:"id"`
				} `json:"musicians"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err != nil || len(resp.Data.Musicians) != 1 || resp.Data.Musicians[0].ID != wantID {
			t.Fatalf("page %d = %s (%v), want musician %d", page+1, w.Body.String(), err, wantID)
		}
	}
}

// spotify_url is derived from the stored id: a matched musician links to its
// Spotify page and an unmatched one carries null, so clients never build the
// URL themselves.
func TestGetMusicianDetails_DerivesSpotifyURL(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	handler := authenticatedRouter(t, app, member.ID)

	tests := []struct {
		name      string
		spotifyID sql.NullString
		wantURL   any
	}{
		{"matched musician", sql.NullString{String: "4Z8W4fKeB5YxbusRsdQVPb", Valid: true}, "https://open.spotify.com/artist/4Z8W4fKeB5YxbusRsdQVPb"},
		{"unmatched musician", sql.NullString{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			musician, err := app.Queries.UpsertMusician(context.Background(), database.UpsertMusicianParams{
				Name:      tt.name,
				SortName:  tt.name,
				SpotifyID: tt.spotifyID,
			})
			if err != nil {
				t.Fatalf("create musician: %v", err)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/music/musicians/%d", musician.ID), nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
			var resp struct {
				Data struct {
					Musician map[string]any `json:"musician"`
				} `json:"data"`
			}
			err = json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("decode response: %v", err)
			}
			got, present := resp.Data.Musician["spotify_url"]
			if !present || got != tt.wantURL {
				t.Fatalf("spotify_url = %v (present %v), want %v", got, present, tt.wantURL)
			}
		})
	}
}
