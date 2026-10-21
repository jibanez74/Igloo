package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"igloo/cmd/internal/helpers"
)

func TestToggleLikeTrack_HTTPPersistsLikeAndUnlike(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Listener", "listener@example.com", false)
	musicianID := createTestMusician(t, app, "Like Artist")
	albumID := createTestAlbum(t, app, "Like Album", "Like Artist")
	trackID := createTestTrack(t, app, "Like Track", "/music/like-track.flac", albumID, musicianID)
	handler := authenticatedRouter(t, app, user.ID)

	toggle := func(t *testing.T) bool {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/music/tracks/%d/like", trackID), nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		assertOpenAPIExchange(t, "toggleLikeTrack", req, w)
		var resp struct {
			Data struct {
				TrackID int64 `json:"track_id"`
				IsLiked bool  `json:"is_liked"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Data.TrackID != trackID {
			t.Fatalf("track_id = %d, want %d", resp.Data.TrackID, trackID)
		}
		return resp.Data.IsLiked
	}
	liked := func(t *testing.T) bool {
		t.Helper()
		ids, err := app.Queries.GetLikedTrackIDsByUserID(context.Background(), user.ID)
		if err != nil {
			t.Fatalf("GetLikedTrackIDsByUserID: %v", err)
		}
		return slices.Contains(ids, trackID)
	}

	if !toggle(t) || !liked(t) {
		t.Fatal("first toggle did not like the track")
	}
	if toggle(t) || liked(t) {
		t.Fatal("second toggle did not remove the like")
	}

	for name, target := range map[string]string{
		"non-numeric id": "/api/music/tracks/abc/like",
		"unknown track":  "/api/music/tracks/999999/like",
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, nil))
			wantStatus, wantMessage := http.StatusBadRequest, invalidTrackIDMessage
			if name == "unknown track" {
				wantStatus, wantMessage = http.StatusNotFound, trackNotFoundMessage
			}
			if w.Code != wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, wantStatus, w.Body.String())
			}
			var resp helpers.JSONResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil || !resp.Error || resp.Message != wantMessage {
				t.Fatalf("response = %s (%v), want %q", w.Body.String(), err, wantMessage)
			}
		})
	}
}

// has_more used to be offset+limit < total, which overflows to a negative
// number at the largest offset the parser accepts and reported more rows for
// an empty library.
func TestGetTracksAlphabetical_HasMore(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Listener", "listener@example.com", false)
	handler := authenticatedRouter(t, app, user.ID)

	type tracksPage struct {
		Data struct {
			Tracks  []json.RawMessage `json:"tracks"`
			Total   int64             `json:"total"`
			Offset  int64             `json:"offset"`
			HasMore bool              `json:"has_more"`
		} `json:"data"`
	}
	fetch := func(t *testing.T, query string) tracksPage {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/music/tracks?"+query, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assertOpenAPIExchange(t, "getTracksAlphabetical", req, w)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
		}
		var page tracksPage
		err := json.Unmarshal(w.Body.Bytes(), &page)
		if err != nil {
			t.Fatal(err)
		}
		return page
	}

	maxOffset := fmt.Sprint(int64(math.MaxInt64))

	empty := fetch(t, "offset="+maxOffset)
	if empty.Data.Total != 0 || len(empty.Data.Tracks) != 0 || empty.Data.HasMore {
		t.Fatalf("empty library at the maximum offset = %+v, want no rows and has_more false", empty.Data)
	}

	seedShuffleTracks(t, app, 3)

	tests := []struct {
		offset      string
		wantTracks  int
		wantHasMore bool
	}{
		{"0", 2, true},
		{"1", 2, false},
		{"2", 1, false},
		{"3", 0, false},
		{"4", 0, false},
		{maxOffset, 0, false},
	}
	for _, tc := range tests {
		t.Run("offset="+tc.offset, func(t *testing.T) {
			page := fetch(t, "limit=2&offset="+tc.offset)
			if page.Data.Total != 3 {
				t.Fatalf("total = %d, want 3", page.Data.Total)
			}
			if len(page.Data.Tracks) != tc.wantTracks || page.Data.HasMore != tc.wantHasMore {
				t.Fatalf("tracks = %d, has_more = %v; want %d and %v", len(page.Data.Tracks), page.Data.HasMore, tc.wantTracks, tc.wantHasMore)
			}
		})
	}
}
