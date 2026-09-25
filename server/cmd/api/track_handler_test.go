package main

import (
	"context"
	"encoding/json"
	"fmt"
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
