package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
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

// The scanner credits a track's primary artist through track_musicians too, so
// the list, the detail page and search all read that table and must agree,
// counting a track the artist only features on once.
func TestMusicianTrackCountsAgreeAcrossListDetailAndSearch(t *testing.T) {
	app := setupSessionTestApp(t)
	member := createTestUser(t, app, "Member", "member@example.com", false)
	handler := authenticatedRouter(t, app, member.ID)

	_, err := app.DB.Exec(`
		INSERT INTO musicians (id, name, sort_name) VALUES (1, 'Aurora', 'aurora'), (2, 'Borealis', 'borealis');
		INSERT INTO tracks (
			id, title, sort_title, file_path, file_name, container, mime_type, codec,
			size, track_index, duration, disc, channels, channel_layout, bit_rate, profile, musician_id
		) VALUES
			(1, 'Sunrise', 'sunrise', '/music/sunrise.flac', 'sunrise.flac', 'flac', 'audio/flac', 'flac', 1, 1, 180, 1, '2', 'stereo', 1000, '', 1),
			(2, 'Daybreak', 'daybreak', '/music/daybreak.flac', 'daybreak.flac', 'flac', 'audio/flac', 'flac', 1, 2, 180, 1, '2', 'stereo', 1000, '', 2);
		INSERT INTO track_musicians (track_id, musician_id) VALUES (1, 1), (2, 2), (2, 1);
	`)
	if err != nil {
		t.Fatalf("seed credits: %v", err)
	}

	type card struct {
		ID         int64 `json:"id"`
		TrackCount int64 `json:"track_count"`
	}
	var resp struct {
		Data struct {
			Musicians []card `json:"musicians"`
			Results   []card `json:"results"`
			Tracks    []struct {
				ID int64 `json:"id"`
			} `json:"tracks"`
		} `json:"data"`
	}
	get := func(t *testing.T, target string) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d: %s", target, w.Code, w.Body.String())
		}
		resp.Data.Musicians, resp.Data.Results, resp.Data.Tracks = nil, nil, nil
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err != nil {
			t.Fatalf("decode %s: %v", target, err)
		}
	}

	get(t, "/api/music/musicians")
	want := []card{{ID: 1, TrackCount: 2}, {ID: 2, TrackCount: 1}}
	if !slices.Equal(resp.Data.Musicians, want) {
		t.Errorf("list = %+v, want %+v", resp.Data.Musicians, want)
	}

	get(t, "/api/search/musicians?q=aurora")
	want = []card{{ID: 1, TrackCount: 2}}
	if !slices.Equal(resp.Data.Results, want) {
		t.Errorf("search = %+v, want %+v", resp.Data.Results, want)
	}

	// sort_title order: Daybreak, then Sunrise.
	get(t, "/api/music/musicians/1")
	var tracks []int64
	for _, track := range resp.Data.Tracks {
		tracks = append(tracks, track.ID)
	}
	if !slices.Equal(tracks, []int64{2, 1}) {
		t.Errorf("detail tracks = %v, want [2 1]", tracks)
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
