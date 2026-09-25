package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

func addMembersToRoom(t *testing.T, app *Application, roomID int64, userIDs ...int64) {
	t.Helper()
	ctx := context.Background()

	for _, id := range userIDs {
		err := app.Queries.AddWatchRoomMember(ctx, database.AddWatchRoomMemberParams{
			RoomID: roomID,
			UserID: id,
		})
		if err != nil {
			t.Fatalf("addMembersToRoom: add member %d: %v", id, err)
		}
	}
}

func TestWatchRoom_OwnerInsertedAsMember(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":null,"invited_user_ids":[]}`, movieID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	rooms, err := app.Queries.GetWatchRoomsForUser(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("GetWatchRoomsForUser failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for owner, got %d", len(rooms))
	}
	room := rooms[0]

	members, err := app.Queries.GetWatchRoomMembers(context.Background(), room.ID)
	if err != nil {
		t.Fatalf("GetWatchRoomMembers failed: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("expected 1 member (owner), got %d", len(members))
	}
	if members[0].ID != ownerID {
		t.Errorf("expected member ID %d (owner), got %d", ownerID, members[0].ID)
	}
}

func TestWatchRoom_InvitedUsersAddedAsMembers(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")

	guest1 := createTestUser(t, app, "Guest One", "guest1@example.com", false)
	guest2 := createTestUser(t, app, "Guest Two", "guest2@example.com", false)

	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"invited_user_ids":[%d,%d]}`, movieID, guest1.ID, guest2.ID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	rooms, err := app.Queries.GetWatchRoomsForUser(ctx, ownerID)
	if err != nil {
		t.Fatalf("GetWatchRoomsForUser failed: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for owner, got %d", len(rooms))
	}
	room := rooms[0]

	members, err := app.Queries.GetWatchRoomMembers(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetWatchRoomMembers failed: %v", err)
	}

	if len(members) != 3 {
		t.Fatalf("expected 3 members (owner + 2 guests), got %d", len(members))
	}
}

func TestWatchRoom_DuplicateMemberRejected(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)

	err := app.Queries.AddWatchRoomMember(ctx, database.AddWatchRoomMemberParams{
		RoomID: room.ID,
		UserID: ownerID,
	})
	if err == nil {
		t.Fatal("expected error inserting duplicate member, got nil")
	}
}

func TestWatchRoom_ListForInvitedUser(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest := createTestUser(t, app, "Guest", "guest@example.com", false)

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	rooms, err := app.Queries.GetWatchRoomsForUser(ctx, guest.ID)
	if err != nil {
		t.Fatalf("GetWatchRoomsForUser for guest failed: %v", err)
	}

	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for invited guest, got %d", len(rooms))
	}
}

func TestWatchRoom_ListExcludesUnrelatedRooms(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	outsider := createTestUser(t, app, "Outsider", "outsider@example.com", false)

	createTestRoom(t, app, ownerID, movieID)

	rooms, err := app.Queries.GetWatchRoomsForUser(ctx, outsider.ID)
	if err != nil {
		t.Fatalf("GetWatchRoomsForUser for outsider failed: %v", err)
	}

	if len(rooms) != 0 {
		t.Fatalf("expected 0 rooms for outsider, got %d", len(rooms))
	}
}

func TestWatchRoom_DeleteRemovesRoomAndMembers(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest := createTestUser(t, app, "Guest", "guest@example.com", false)

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	err := app.Queries.DeleteWatchRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("DeleteWatchRoom failed: %v", err)
	}

	_, err = app.Queries.GetWatchRoomByID(ctx, room.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected room to be deleted (sql.ErrNoRows), got: %v", err)
	}

	members, err := app.Queries.GetWatchRoomMembers(ctx, room.ID)
	if err != nil {
		t.Fatalf("GetWatchRoomMembers after delete failed: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("expected 0 members after room delete, got %d", len(members))
	}
}

func TestWatchRoom_CascadeDeleteMovie(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)

	err := app.Queries.DeleteMovie(ctx, movieID)
	if err != nil {
		t.Fatalf("DeleteMovie failed: %v", err)
	}

	_, err = app.Queries.GetWatchRoomByID(ctx, room.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected room to be cascade-deleted with movie, got: %v", err)
	}
}

func TestWatchRoom_CascadeDeleteUser(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)

	err := app.Queries.DeleteUser(ctx, ownerID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	_, err = app.Queries.GetWatchRoomByID(ctx, room.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected room to be cascade-deleted with owner user, got: %v", err)
	}
}

func TestDeduplicateAndFilterUserIDs(t *testing.T) {
	tests := []struct {
		name     string
		ids      []int64
		ownerID  int64
		expected []int64
	}{
		{"empty list", []int64{}, 1, []int64{}},
		{"owner excluded", []int64{1, 2, 3}, 1, []int64{2, 3}},
		{"duplicates removed", []int64{2, 2, 3}, 1, []int64{2, 3}},
		{"owner and duplicate", []int64{1, 2, 1, 2}, 1, []int64{2}},
		{"all excluded", []int64{1, 1}, 1, []int64{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateAndFilterUserIDs(tt.ids, tt.ownerID)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i, id := range got {
				if id != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], id)
				}
			}
		})
	}
}

func TestIsValidPlaybackMode(t *testing.T) {
	tests := []struct {
		mode  string
		valid bool
	}{
		{"direct", true},
		{"remux", true},
		{"1080p_8mbps", true},
		{"720p_3mbps", true},
		{"", false},
		{"unknown", false},
		{"hls", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			if got := isValidPlaybackMode(tt.mode); got != tt.valid {
				t.Errorf("isValidPlaybackMode(%q) = %v, want %v", tt.mode, got, tt.valid)
			}
		})
	}
}

func performWatchRoomHTTPRequest(t *testing.T, app *Application, userID int64, method string, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	authenticatedRouter(t, app, userID).ServeHTTP(w, req)
	return w
}

// createTestRoom builds the common direct-playback room.
func createTestRoom(t *testing.T, app *Application, ownerID, movieID int64) database.WatchRoom {
	t.Helper()

	return createTestRoomWithMode(t, app, ownerID, movieID, "direct")
}

func createTestRoomWithMode(t *testing.T, app *Application, ownerID, movieID int64, mode string) database.WatchRoom {
	t.Helper()

	room, err := app.Queries.CreateWatchRoom(context.Background(), database.CreateWatchRoomParams{
		OwnerUserID:  ownerID,
		MovieID:      movieID,
		PlaybackMode: mode,
		AudioTrack:   0,
	})
	if err != nil {
		t.Fatalf("create room with mode %q: %v", mode, err)
	}

	return room
}

func TestCreateWatchRoom_HTTP_Success(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":null,"invited_user_ids":[]}`, movieID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "createWatchRoom", req, w)

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error {
		t.Errorf("expected error=false, got error=true: %s", resp.Message)
	}
}

func TestCreateWatchRoom_HTTP_InvalidMode(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"badmode","audio_track":0}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateWatchRoom_HTTP_NegativeAudioTrackRejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":-1}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDirectPlayAudioSelectionUnambiguous(t *testing.T) {
	stream := func(isDefault bool) database.AudioStream {
		return database.AudioStream{IsDefault: isDefault}
	}

	cases := []struct {
		name    string
		streams []database.AudioStream
		want    bool
	}{
		{name: "no streams", streams: nil, want: true},
		{name: "single stream", streams: []database.AudioStream{stream(false)}, want: true},
		{name: "multiple streams, no defaults", streams: []database.AudioStream{stream(false), stream(false)}, want: true},
		{name: "single default on stream 0", streams: []database.AudioStream{stream(true), stream(false)}, want: true},
		{name: "single default on non-zero index", streams: []database.AudioStream{stream(false), stream(true)}, want: false},
		{name: "multiple defaults", streams: []database.AudioStream{stream(true), stream(true)}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := directPlayAudioSelectionUnambiguous(tc.streams)
			if got != tc.want {
				t.Errorf("directPlayAudioSelectionUnambiguous = %v, want %v", got, tc.want)
			}
		})
	}
}

func insertWatchRoomTestAudioStream(t *testing.T, app *Application, movieID int64, streamIndex int64, isDefault bool) {
	t.Helper()

	err := app.Queries.InsertAudioStream(context.Background(), database.InsertAudioStreamParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
		Codec:       "aac",
		Channels:    2,
		IsDefault:   isDefault,
	})
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}
}

func insertWatchRoomTestVideoStream(t *testing.T, app *Application, movieID int64, codecProfile string) {
	t.Helper()

	err := app.Queries.InsertVideoStream(context.Background(), database.InsertVideoStreamParams{
		MovieID:      movieID,
		StreamIndex:  0,
		Codec:        "h264",
		CodecProfile: sql.NullString{String: codecProfile, Valid: codecProfile != ""},
		Width:        1920,
		Height:       1080,
	})
	if err != nil {
		t.Fatalf("insert video stream: %v", err)
	}
}

func TestCreateWatchRoom_HTTP_DirectForNonMP4Rejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, _ := createTestUserAndMovie(t, app)
	mkvMovieID, err := app.Queries.UpsertMovie(context.Background(), database.UpsertMovieParams{
		Title:     "Matroska Movie",
		FilePath:  "/movies/matroska.mkv",
		FileName:  "matroska.mkv",
		Size:      2048,
		Container: "mkv",
		MimeType:  helpers.VideoMimeTypes["mkv"],
	})
	if err != nil {
		t.Fatalf("insert mkv movie: %v", err)
	}
	mkvMovie, err := app.Queries.GetMovieByID(context.Background(), mkvMovieID)
	if err != nil {
		t.Fatal(err)
	}
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":null,"invited_user_ids":[]}`, mkvMovie.ID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "createWatchRoom", req, w)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a direct room on a non-MP4 movie, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWatchRoom_HTTP_DirectWithNonBrowserSafeH264Rejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High 10")
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":null,"invited_user_ids":[]}`, movieID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "createWatchRoom", req, w)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a direct room on a High 10 movie, got %d: %s", w.Code, w.Body.String())
	}
}

// An embedded poster is stored as a video stream, and in some files it sorts
// ahead of the feature. The gate must judge the feature, exactly as the web
// client's getPrimaryVideoStream does, or the room is refused for a movie that
// direct-plays fine.
func TestCreateWatchRoom_HTTP_DirectSkipsCoverArtVideoStream(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)

	err := app.Queries.InsertVideoStream(context.Background(), database.InsertVideoStreamParams{
		MovieID:     movieID,
		StreamIndex: 0,
		Codec:       "mjpeg",
		Width:       600,
		Height:      900,
	})
	if err != nil {
		t.Fatalf("insert cover art stream: %v", err)
	}

	err = app.Queries.InsertVideoStream(context.Background(), database.InsertVideoStreamParams{
		MovieID:      movieID,
		StreamIndex:  1,
		Codec:        "h264",
		CodecProfile: sql.NullString{String: "High", Valid: true},
		Width:        1920,
		Height:       1080,
	})
	if err != nil {
		t.Fatalf("insert feature video stream: %v", err)
	}

	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 for a direct room whose first video stream is cover art, got %d: %s", w.Code, w.Body.String())
	}
}

// Audit matrix row 18b (D17), server mirror: a movie whose scan produced no
// video streams cannot be direct-played, so a direct room must be refused.
func TestCreateWatchRoom_HTTP_DirectWithNoVideoStreamsRejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":null,"invited_user_ids":[]}`, movieID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "createWatchRoom", req, w)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a direct room on a movie with no video streams, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWatchRoom_HTTP_DirectWithAmbiguousAudioRejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestAudioStream(t, app, movieID, 1, false)
	insertWatchRoomTestAudioStream(t, app, movieID, 2, true)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":null,"invited_user_ids":[]}`, movieID)
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "createWatchRoom", req, w)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for direct mode with a non-first default audio stream, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWatchRoom_HTTP_DirectWithFirstStreamDefaultAccepted(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")
	insertWatchRoomTestAudioStream(t, app, movieID, 1, true)
	insertWatchRoomTestAudioStream(t, app, movieID, 2, false)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 for direct mode with the default on stream 0, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWatchRoom_HTTP_NegativeSubtitleTrackRejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":-1}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateWatchRoom_HTTP_MovieNotFound(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := `{"movie_id":99999,"mode":"direct","audio_track":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateWatchRoom_HTTP_InvalidInvitedUser(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"invited_user_ids":[99999]}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for nonexistent invited user, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWatchRoom_HTTP_OwnerInviteDeduplication(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"invited_user_ids":[%d]}`, movieID, ownerID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	rooms, err := app.Queries.GetWatchRoomsForUser(ctx, ownerID)
	if err != nil {
		t.Fatalf("GetWatchRoomsForUser: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}

	members, err := app.Queries.GetWatchRoomMembers(ctx, rooms[0].ID)
	if err != nil {
		t.Fatalf("GetWatchRoomMembers: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member (owner once), got %d", len(members))
	}
}

func TestWatchRoom_HTTP_ProductionRouterRequiresAuthentication(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session cookie, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetWatchRooms_HTTP_ProductionRouterResponseShape(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	_, err := app.DB.ExecContext(ctx, `UPDATE movies SET poster_path = ? WHERE id = ?`, "/poster.jpg", movieID)
	if err != nil {
		t.Fatalf("update movie poster: %v", err)
	}
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Invited Guest",
		Email:    "invited-shape@example.com",
		Password: "hashed",
		Avatar:   sql.NullString{String: "avatars/guest.webp", Valid: true},
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms", nil)
	req.AddCookie(newAuthSessionCookie(t, app, ownerID))
	w := httptest.NewRecorder()
	app.InitRouter()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getWatchRooms", req, w)

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp.Data.(map[string]any)
	rooms := data["rooms"].([]any)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}

	gotRoom := rooms[0].(map[string]any)
	if gotRoom["movie_title"] != "Test Movie" {
		t.Fatalf("movie_title = %#v, want Test Movie", gotRoom["movie_title"])
	}
	if gotRoom["movie_poster"] != "/poster.jpg" {
		t.Fatalf("movie_poster = %#v, want poster path", gotRoom["movie_poster"])
	}
	if gotRoom["is_owner"] != true {
		t.Fatalf("expected is_owner=true, got %#v", gotRoom["is_owner"])
	}
	members := gotRoom["members"].([]any)
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	if members[1].(map[string]any)["avatar"] != "avatars/guest.webp" {
		t.Fatalf("expected guest avatar in members response, got %#v", members[1])
	}
}

func TestGetWatchRoom_HTTP_DetailIncludesPlaybackAndNullableFields(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	_, err := app.DB.ExecContext(ctx, `UPDATE movies SET poster_path = ? WHERE id = ?`, "/detail-poster.jpg", movieID)
	if err != nil {
		t.Fatalf("update movie poster: %v", err)
	}

	room, err := app.Queries.CreateWatchRoom(ctx, database.CreateWatchRoomParams{
		OwnerUserID:   ownerID,
		MovieID:       movieID,
		PlaybackMode:  helpers.HLS_PROFILE_720P_3MBPS,
		AudioTrack:    2,
		SubtitleTrack: sql.NullInt64{Int64: 3, Valid: true},
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	addMembersToRoom(t, app, room.ID, ownerID)

	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	detail := resp.Data.(map[string]any)["room"].(map[string]any)
	if detail["playback_mode"] != helpers.HLS_PROFILE_720P_3MBPS {
		t.Fatalf("playback_mode = %#v", detail["playback_mode"])
	}
	if detail["audio_track"] != float64(2) {
		t.Fatalf("audio_track = %#v, want 2", detail["audio_track"])
	}
	if detail["subtitle_track"] != float64(3) {
		t.Fatalf("subtitle_track = %#v, want 3", detail["subtitle_track"])
	}
	if detail["movie_poster"] != "/detail-poster.jpg" {
		t.Fatalf("movie_poster = %#v, want /detail-poster.jpg", detail["movie_poster"])
	}
	if detail["is_owner"] != true {
		t.Fatalf("expected is_owner=true, got %#v", detail["is_owner"])
	}
}

func TestCreateWatchRoom_HTTP_HLSWarmUpStoresRoomSession(t *testing.T) {
	app := setupSessionTestApp(t)
	app.SetSettings(&database.Setting{})
	app.FFmpeg = &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}

	owner := createTestUser(t, app, "HLS Owner", "hls-owner@example.com", false)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 720)
	handler := authenticatedRouter(t, app, owner.ID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"%s","audio_track":0,"invited_user_ids":[]}`, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	roomID := int64(resp.Data.(map[string]any)["room_id"].(float64))
	if _, ok := app.HLSSessionCache.Get(RoomHLSSessionKey(roomID)); !ok {
		t.Fatal("expected warmed room HLS session to be stored in cache")
	}
	if app.FFmpeg.(*fakeFFmpeg).CallCount() != 1 {
		t.Fatalf("expected one FFmpeg HLS warm-up call, got %d", app.FFmpeg.(*fakeFFmpeg).CallCount())
	}
}

func TestCreateWatchRoom_HTTP_HLSWarmUpFailureRollsBackRoom(t *testing.T) {
	app := setupSessionTestApp(t)
	app.SetSettings(&database.Setting{})

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"%s","audio_track":0,"invited_user_ids":[]}`, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 from HLS warm-up failure, got %d: %s", w.Code, w.Body.String())
	}

	rooms, err := app.Queries.GetWatchRoomsForUser(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("get rooms after rollback: %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("expected no rooms after failed HLS warm-up rollback, got %d", len(rooms))
	}
}

// An unknown room is indistinguishable from a room the caller cannot see: the
// single joined authorization query returns no row either way, so both are 403.
// The other room endpoints have always behaved this way.
func TestGetWatchRoom_HTTP_UnknownRoomIsForbidden(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms/99999", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestGetWatchRoom_HTTP_ForbiddenForNonMember(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider := createTestUser(t, app, "Outsider", "outsider@example.com", false)

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, outsider.ID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member, got %d", w.Code)
	}
}

func TestGetWatchRoom_HTTP_SuccessForMember(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getWatchRoom", req, w)
}

func TestJoinWatchRoom_HTTP_SuccessForMember(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/watch-rooms/%d/join", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "joinWatchRoom", req, w)
}

func TestJoinWatchRoom_HTTP_ForbiddenForNonMember(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider := createTestUser(t, app, "Outsider", "outsider@example.com", false)

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, outsider.ID)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/watch-rooms/%d/join", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member join, got %d", w.Code)
	}
}

func TestDeleteWatchRoom_HTTP_SuccessForOwner(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "deleteWatchRoom", req, w)

	_, err := app.Queries.GetWatchRoomByID(context.Background(), room.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected room to be deleted, got: %v", err)
	}
}

func TestDeleteWatchRoom_HTTP_InvalidatesCachedAuthorization(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoomWithMode(t, app, ownerID, movieID, watchRoomPlaybackModeDirect)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, ownerID)
	detailPath := fmt.Sprintf("/api/watch-rooms/%d", room.ID)
	streamPath := fmt.Sprintf("/api/watch-rooms/%d/stream", room.ID)

	warmRequest := httptest.NewRequest(http.MethodGet, detailPath, nil)
	warmResponse := httptest.NewRecorder()
	handler.ServeHTTP(warmResponse, warmRequest)
	if warmResponse.Code != http.StatusOK {
		t.Fatalf("warm detail request returned %d: %s", warmResponse.Code, warmResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, detailPath, nil)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete returned %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}

	for _, path := range []string{detailPath, streamPath} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Errorf("GET %s returned %d after delete, want 403: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestDeleteWatchRoom_HTTP_CleansUpRoomHLSSession(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := authenticatedRouter(t, app, ownerID)

	app.HLSSessionCache.SetDefault(RoomHLSSessionKey(room.ID), &HLSSession{
		TempDir: "",
	})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, ok := app.HLSSessionCache.Get(RoomHLSSessionKey(room.ID)); ok {
		t.Fatal("expected room HLS session cache entry to be removed after delete")
	}
}

func TestDeleteWatchRoom_HTTP_ForbiddenForInvitedMember(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "guest@example.com", false)

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	handler := authenticatedRouter(t, app, guest.ID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for invited non-owner delete, got %d", w.Code)
	}
}

func TestDeleteWatchRoom_HTTP_NotFound(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := authenticatedRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodDelete, "/api/watch-rooms/99999", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestStreamWatchRoomMovie_HTTP_DirectStreamUsesRealMembershipAndMode(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()

	moviePath := filepath.Join(t.TempDir(), "watch-room-direct.mp4")
	const body = "watch room direct stream fixture"
	if err := os.WriteFile(moviePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write movie fixture: %v", err)
	}

	ownerID, movieID := createTestUserAndMovie(t, app)
	_, err := app.DB.ExecContext(ctx, `
		UPDATE movies
		SET file_path = ?, file_name = ?, container = 'mp4', mime_type = 'video/mp4', size = ?
		WHERE id = ?
	`, moviePath, filepath.Base(moviePath), len(body), movieID)
	if err != nil {
		t.Fatalf("update movie path: %v", err)
	}
	outsider := createTestUser(t, app, "Stream Outsider", "stream-outsider@example.com", false)

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)

	path := fmt.Sprintf("/api/watch-rooms/%d/stream", room.ID)
	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, path)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for direct stream, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != body {
		t.Fatalf("stream body = %q, want %q", got, body)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "video/mp4") {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}
	if got := w.Header().Get("ETag"); got == "" {
		t.Fatal("direct stream response has no ETag")
	}

	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodHead, path)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for direct stream HEAD, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Length"); got != fmt.Sprintf("%d", len(body)) {
		t.Fatalf("HEAD Content-Length = %q, want %d", got, len(body))
	}
	if w.Body.Len() != 0 {
		t.Fatalf("HEAD body = %d bytes, want empty", w.Body.Len())
	}

	w = performWatchRoomHTTPRequest(t, app, outsider.ID, http.MethodGet, path)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member stream, got %d: %s", w.Code, w.Body.String())
	}

	hlsRoom := createTestRoomWithMode(t, app, ownerID, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	addMembersToRoom(t, app, hlsRoom.ID, ownerID)
	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/stream", hlsRoom.ID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when streaming an HLS room directly, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStreamWatchRoomMovie_HTTP_DeletedMovieReturns404(t *testing.T) {
	app := setupSessionTestApp(t)
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)

	// Deleting a movie normally cascades to its rooms; suspend FK enforcement
	// to simulate a room left pointing at a movie row that no longer exists.
	// The in-memory test DB has a single pooled connection, so sequential
	// Execs land on the same connection the pragma applies to.
	_, err := app.DB.ExecContext(ctx, `PRAGMA foreign_keys = OFF`)
	if err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}
	_, err = app.DB.ExecContext(ctx, `DELETE FROM movies WHERE id = ?`, movieID)
	if err != nil {
		t.Fatalf("delete movie: %v", err)
	}
	_, err = app.DB.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	if err != nil {
		t.Fatalf("re-enable foreign keys: %v", err)
	}

	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/stream", room.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a room whose movie is gone, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWatchRoomHLS_HTTP_RequiresMembershipModeAndManifestBeforeSegments(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider := createTestUser(t, app, "HLS Outsider", "hls-outsider@example.com", false)

	directRoom := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, directRoom.ID, ownerID)
	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/playlist.m3u8", directRoom.ID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for HLS manifest on direct room, got %d: %s", w.Code, w.Body.String())
	}

	hlsRoom := createTestRoomWithMode(t, app, ownerID, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	addMembersToRoom(t, app, hlsRoom.ID, ownerID)

	w = performWatchRoomHTTPRequest(t, app, outsider.ID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/segment_0.m4s", hlsRoom.ID))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member HLS segment, got %d: %s", w.Code, w.Body.String())
	}

	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/bad_name.m4s", hlsRoom.ID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid segment request to be rejected, got %d: %s", w.Code, w.Body.String())
	}

	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/segment_0.m4s", hlsRoom.ID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before manifest/session creation, got %d: %s", w.Code, w.Body.String())
	}
}

// insertWatchRoomAudioStreams gives a movie `count` audio streams whose absolute
// ffprobe indices deliberately start at 1, so an ordinal is never its own index.
func insertWatchRoomAudioStreams(t *testing.T, app *Application, movieID int64, count int) {
	t.Helper()
	ctx := context.Background()
	languages := []string{"eng", "spa", "fra"}

	for i := 0; i < count; i++ {
		err := app.Queries.InsertAudioStream(ctx, database.InsertAudioStreamParams{
			MovieID:     movieID,
			StreamIndex: int64(i + 1),
			Codec:       "aac",
			BitRate:     192000,
			Channels:    2,
			Language:    sql.NullString{String: languages[i%len(languages)], Valid: true},
		})
		if err != nil {
			t.Fatalf("insert audio stream %d: %v", i, err)
		}
	}
}

func TestCreateWatchRoom_HTTP_AudioTrackValidation(t *testing.T) {
	tests := []struct {
		name       string
		audioCount int
		mode       string
		audioTrack int
		wantStatus int
	}{
		{name: "first track accepted for direct", audioCount: 3, mode: "direct", audioTrack: 0, wantStatus: http.StatusCreated},
		{name: "out of range rejected", audioCount: 2, mode: "remux", audioTrack: 2, wantStatus: http.StatusBadRequest},
		{name: "non first track rejected for direct", audioCount: 3, mode: "direct", audioTrack: 1, wantStatus: http.StatusBadRequest},
		{name: "non zero track rejected without audio", audioCount: 0, mode: "remux", audioTrack: 1, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupSessionTestApp(t)

			ownerID, movieID := createTestUserAndMovie(t, app)
			insertWatchRoomTestVideoStream(t, app, movieID, "High")
			insertWatchRoomAudioStreams(t, app, movieID, tt.audioCount)
			handler := authenticatedRouter(t, app, ownerID)

			body := fmt.Sprintf(`{"movie_id":%d,"mode":%q,"audio_track":%d,"subtitle_track":null,"invited_user_ids":[]}`, movieID, tt.mode, tt.audioTrack)
			req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assertOpenAPIExchange(t, "createWatchRoom", req, w)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

// An in-range non-first track on a transcoding mode must survive validation and
// reach warm-up, which is the combination the settings dialog now produces.
func TestCreateWatchRoom_HTTP_NonFirstAudioTrackAcceptedForHLS(t *testing.T) {
	app := setupSessionTestApp(t)
	app.SetSettings(&database.Setting{})
	app.FFmpeg = &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}

	owner := createTestUser(t, app, "Multi Audio Owner", "multi-audio-owner@example.com", false)

	movieID := insertTestHLSMovieFixture(t, app, "h264", 720)
	// The fixture already holds one audio stream at index 1; add a second so
	// ordinal 1 resolves to absolute ffprobe index 2.
	_, err := app.DB.Exec(`
		INSERT INTO audio_streams (movie_id, stream_index, codec, bit_rate, channels, language)
		VALUES (?, ?, ?, ?, ?, ?)
	`, movieID, 2, "aac", 192000, 2, "spa")
	if err != nil {
		t.Fatalf("insert second audio stream: %v", err)
	}

	handler := authenticatedRouter(t, app, owner.ID)
	body := fmt.Sprintf(`{"movie_id":%d,"mode":"%s","audio_track":1,"invited_user_ids":[]}`, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	roomID := int64(resp.Data.(map[string]any)["room_id"].(float64))
	storedOwnerID, err := app.Queries.GetWatchRoomByID(context.Background(), roomID)
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	room, err := app.Queries.GetWatchRoomForMember(context.Background(), database.GetWatchRoomForMemberParams{ID: roomID, UserID: storedOwnerID})
	if err != nil {
		t.Fatal(err)
	}
	if room.AudioTrack != 1 {
		t.Fatalf("expected stored audio_track 1, got %d", room.AudioTrack)
	}
}

func insertWatchRoomTestSubtitle(t *testing.T, app *Application, movieID int64, streamIndex int64, language string) {
	t.Helper()

	err := app.Queries.InsertSubtitle(context.Background(), database.InsertSubtitleParams{
		MovieID:     movieID,
		StreamIndex: streamIndex,
		Codec:       "subrip",
		Language:    sql.NullString{String: language, Valid: language != ""},
	})
	if err != nil {
		t.Fatalf("insert subtitle: %v", err)
	}
}

func TestCreateWatchRoom_HTTP_SubtitleTrackOutOfRangeRejected(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")
	handler := authenticatedRouter(t, app, ownerID)

	postRoom := func(body string) *httptest.ResponseRecorder {
		req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assertOpenAPIExchange(t, "createWatchRoom", req, w)
		return w
	}

	noSubtitles := postRoom(fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":0,"invited_user_ids":[]}`, movieID))
	if noSubtitles.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a movie without subtitles, got %d: %s", noSubtitles.Code, noSubtitles.Body.String())
	}

	insertWatchRoomTestSubtitle(t, app, movieID, 2, "eng")
	outOfRange := postRoom(fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":3,"invited_user_ids":[]}`, movieID))
	if outOfRange.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range subtitle_track, got %d: %s", outOfRange.Code, outOfRange.Body.String())
	}

	valid := postRoom(fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":0,"invited_user_ids":[]}`, movieID))
	if valid.Code != http.StatusCreated {
		t.Fatalf("expected 201 for in-range subtitle_track, got %d: %s", valid.Code, valid.Body.String())
	}
}

func TestCreateWatchRoom_PersistsStreamPins(t *testing.T) {
	app := setupSessionTestApp(t)

	ownerID, movieID := createTestUserAndMovie(t, app)
	insertWatchRoomTestVideoStream(t, app, movieID, "High")
	err := app.Queries.InsertAudioStream(context.Background(), database.InsertAudioStreamParams{
		MovieID:     movieID,
		StreamIndex: 1,
		Codec:       "aac",
		Channels:    2,
		Language:    sql.NullString{String: "eng", Valid: true},
	})
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}
	insertWatchRoomTestSubtitle(t, app, movieID, 3, "spa")
	handler := authenticatedRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"subtitle_track":0}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	roomID := int64(resp.Data.(map[string]any)["room_id"].(float64))

	storedOwnerID, err := app.Queries.GetWatchRoomByID(context.Background(), roomID)
	if err != nil {
		t.Fatalf("load room: %v", err)
	}
	room, err := app.Queries.GetWatchRoomForMember(context.Background(), database.GetWatchRoomForMemberParams{ID: roomID, UserID: storedOwnerID})
	if err != nil {
		t.Fatal(err)
	}
	if !room.AudioStreamIndex.Valid || room.AudioStreamIndex.Int64 != 1 {
		t.Errorf("audio_stream_index = %+v, want 1", room.AudioStreamIndex)
	}
	if !room.AudioLanguage.Valid || room.AudioLanguage.String != "eng" {
		t.Errorf("audio_language = %+v, want eng", room.AudioLanguage)
	}
	if !room.SubtitleStreamIndex.Valid || room.SubtitleStreamIndex.Int64 != 3 {
		t.Errorf("subtitle_stream_index = %+v, want 3", room.SubtitleStreamIndex)
	}
	if !room.SubtitleLanguage.Valid || room.SubtitleLanguage.String != "spa" {
		t.Errorf("subtitle_language = %+v, want spa", room.SubtitleLanguage)
	}
}

func TestVerifyWatchRoomStreamPins(t *testing.T) {
	app := setupSessionTestApp(t)

	_, movieID := createTestUserAndMovie(t, app)
	ctx := context.Background()
	err := app.Queries.InsertAudioStream(ctx, database.InsertAudioStreamParams{
		MovieID:     movieID,
		StreamIndex: 1,
		Codec:       "aac",
		Channels:    2,
		Language:    sql.NullString{String: "eng", Valid: true},
	})
	if err != nil {
		t.Fatalf("insert audio stream: %v", err)
	}
	insertWatchRoomTestSubtitle(t, app, movieID, 5, "eng")

	pinInt := func(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
	pinStr := func(v string) sql.NullString { return sql.NullString{String: v, Valid: true} }

	cases := []struct {
		name      string
		room      database.WatchRoom
		wantDrift bool
	}{
		{
			name: "unpinned room skips every check",
			room: database.WatchRoom{MovieID: movieID, AudioTrack: 9},
		},
		{
			name: "matching pins pass",
			room: database.WatchRoom{
				MovieID: movieID, AudioTrack: 0,
				AudioStreamIndex: pinInt(1), AudioLanguage: pinStr("eng"),
				SubtitleTrack:       pinInt(0),
				SubtitleStreamIndex: pinInt(5), SubtitleLanguage: pinStr("eng"),
			},
		},
		{
			name: "audio ordinal out of range drifts",
			room: database.WatchRoom{
				MovieID: movieID, AudioTrack: 4,
				AudioStreamIndex: pinInt(1),
			},
			wantDrift: true,
		},
		{
			name: "audio stream index mismatch drifts",
			room: database.WatchRoom{
				MovieID: movieID, AudioTrack: 0,
				AudioStreamIndex: pinInt(2),
			},
			wantDrift: true,
		},
		{
			name: "audio language mismatch drifts",
			room: database.WatchRoom{
				MovieID: movieID, AudioTrack: 0,
				AudioStreamIndex: pinInt(1), AudioLanguage: pinStr("jpn"),
			},
			wantDrift: true,
		},
		{
			name: "null pinned language is not checked",
			room: database.WatchRoom{
				MovieID: movieID, AudioTrack: 0,
				AudioStreamIndex: pinInt(1),
			},
		},
		{
			name: "subtitle stream index mismatch drifts",
			room: database.WatchRoom{
				MovieID: movieID, AudioTrack: 0,
				SubtitleTrack:       pinInt(0),
				SubtitleStreamIndex: pinInt(9),
			},
			wantDrift: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := app.verifyWatchRoomStreamPins(ctx, tc.room)
			if tc.wantDrift {
				if !errors.Is(err, errWatchRoomStreamDrift) {
					t.Fatalf("expected stream drift, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected pins to verify, got %v", err)
			}
		})
	}
}

func TestWatchRoomHLSManifest_ReturnsConflictOnStreamDrift(t *testing.T) {
	app := setupSessionTestApp(t)
	app.SetSettings(&database.Setting{})
	app.FFmpeg = &fakeFFmpeg{
		plans: []fakeFFmpegRunPlan{
			{
				WriteFiles: func(outDir string) error {
					return writeTestHLSFixture(outDir, testFMP4Fixture{
						SafeVideo: true,
						Segments:  1,
					})
				},
			},
		},
	}

	owner := createTestUser(t, app, "Drift Owner", "drift-owner@example.com", false)
	movieID := insertTestHLSMovieFixture(t, app, "h264", 720)
	handler := authenticatedRouter(t, app, owner.ID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"%s","audio_track":0,"invited_user_ids":[]}`, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	roomID := int64(resp.Data.(map[string]any)["room_id"].(float64))

	manifestPath := fmt.Sprintf("/api/watch-rooms/%d/hls/playlist.m3u8", roomID)
	healthyRequest := httptest.NewRequest(http.MethodGet, manifestPath, nil)
	healthy := httptest.NewRecorder()
	handler.ServeHTTP(healthy, healthyRequest)
	if healthy.Code != http.StatusOK {
		t.Fatalf("expected 200 before drift, got %d: %s", healthy.Code, healthy.Body.String())
	}
	assertOpenAPIExchange(t, "watchRoomHLSManifest", healthyRequest, healthy)

	assetPath := fmt.Sprintf("/api/watch-rooms/%d/hls/segment_0.m4s?audio_track=0", roomID)
	assetRequest := httptest.NewRequest(http.MethodGet, assetPath, nil)
	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, assetRequest)
	if asset.Code != http.StatusOK {
		t.Fatalf("expected room HLS asset 200, got %d: %s", asset.Code, asset.Body.String())
	}
	assertOpenAPIExchange(t, "watchRoomHLSSegment", assetRequest, asset)

	// Simulate a rescan of a replaced file whose track layout differs: the
	// ordinal still resolves, but to a different absolute stream index.
	_, err := app.DB.Exec(`UPDATE audio_streams SET stream_index = 2 WHERE movie_id = ?`, movieID)
	if err != nil {
		t.Fatalf("shift audio stream index: %v", err)
	}
	// A real rescan ends by calling this; the raw UPDATE above has to do the
	// same or the pin check keeps reading the pre-drift streams from cache.
	app.invalidateCommittedMovie(movieID)

	driftedRequest := httptest.NewRequest(http.MethodGet, manifestPath, nil)
	drifted := httptest.NewRecorder()
	handler.ServeHTTP(drifted, driftedRequest)
	assertOpenAPIExchange(t, "watchRoomHLSManifest", driftedRequest, drifted)
	if drifted.Code != http.StatusConflict {
		t.Fatalf("expected 409 after drift, got %d: %s", drifted.Code, drifted.Body.String())
	}
	if !strings.Contains(drifted.Body.String(), "delete the room and create it again") {
		t.Fatalf("expected actionable drift message, got %s", drifted.Body.String())
	}
}

func TestWatchRoomMediaSubtitleDriftConformsToOpenAPI(t *testing.T) {
	for _, mutation := range []struct{ name, sql string }{
		{"removed subtitle", `DELETE FROM subtitles WHERE movie_id = ?`},
		{"changed stream index", `UPDATE subtitles SET stream_index = 6 WHERE movie_id = ?`},
		{"changed language", `UPDATE subtitles SET language = 'jpn' WHERE movie_id = ?`},
	} {
		for _, mode := range []string{"direct", helpers.HLS_PROFILE_720P_3MBPS} {
			t.Run(mode+"/"+mutation.name, func(t *testing.T) {
				app := setupSessionTestApp(t)
				ownerID, movieID := createTestUserAndMovie(t, app)
				insertWatchRoomTestSubtitle(t, app, movieID, 5, "eng")
				room := createTestRoomWithMode(t, app, ownerID, movieID, mode)
				addMembersToRoom(t, app, room.ID, ownerID)
				_, err := app.DB.Exec(`UPDATE watch_rooms SET subtitle_track = 0, subtitle_stream_index = 5, subtitle_language = 'eng' WHERE id = ?`, room.ID)
				if err != nil {
					t.Fatal(err)
				}
				_, err = app.DB.Exec(mutation.sql, movieID)
				if err != nil {
					t.Fatal(err)
				}
				app.invalidateCommittedMovie(movieID)
				path := fmt.Sprintf("/api/watch-rooms/%d/stream", room.ID)
				operation := "streamWatchRoomMovie"
				if mode != "direct" {
					path = fmt.Sprintf("/api/watch-rooms/%d/hls/playlist.m3u8", room.ID)
					operation = "watchRoomHLSManifest"
				}
				request := httptest.NewRequest(http.MethodGet, path, nil)
				response := httptest.NewRecorder()
				authenticatedRouter(t, app, ownerID).ServeHTTP(response, request)
				if response.Code != http.StatusConflict {
					t.Fatalf("status = %d: %s", response.Code, response.Body.String())
				}
				assertOpenAPIExchange(t, operation, request, response)
			})
		}
	}
}
