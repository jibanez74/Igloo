package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func createTestRoom(t *testing.T, app *Application, ownerID, movieID int64) database.WatchRoom {
	t.Helper()
	ctx := context.Background()
	room, err := app.Queries.CreateWatchRoom(ctx, database.CreateWatchRoomParams{
		OwnerUserID:  ownerID,
		MovieID:      movieID,
		PlaybackMode: "direct",
		AudioTrack:   0,
	})
	if err != nil {
		t.Fatalf("createTestRoom: create room: %v", err)
	}

	return room
}

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

func setupWatchRoomHTTPTestApp(t *testing.T) *Application {
	t.Helper()
	app := setupTestApp(t)
	app.InitSession()
	return app
}

func TestWatchRoom_CreateAndFetch(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)

	room := createTestRoom(t, app, ownerID, movieID)

	if room.OwnerUserID != ownerID {
		t.Errorf("expected owner_user_id %d, got %d", ownerID, room.OwnerUserID)
	}
	if room.MovieID != movieID {
		t.Errorf("expected movie_id %d, got %d", movieID, room.MovieID)
	}
	if room.PlaybackMode != "direct" {
		t.Errorf("expected playback_mode 'direct', got %q", room.PlaybackMode)
	}

	fetched, err := app.Queries.GetWatchRoomByID(context.Background(), room.ID)
	if err != nil {
		t.Fatalf("GetWatchRoomByID failed: %v", err)
	}
	if fetched.ID != room.ID {
		t.Errorf("expected room ID %d, got %d", room.ID, fetched.ID)
	}
}

func TestWatchRoom_OwnerInsertedAsMember(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"invited_user_ids":[]}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest1, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest One",
		Email:    "guest1@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest1: %v", err)
	}
	guest2, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest Two",
		Email:    "guest2@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest2: %v", err)
	}

	handler := mountWatchRoomRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"invited_user_ids":[%d,%d]}`, movieID, guest1.ID, guest2.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	defer app.DB.Close()
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

func TestWatchRoom_ListForOwner(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)

	rooms, err := app.Queries.GetWatchRoomsForUser(context.Background(), ownerID)
	if err != nil {
		t.Fatalf("GetWatchRoomsForUser failed: %v", err)
	}

	if len(rooms) != 1 {
		t.Fatalf("expected 1 room for owner, got %d", len(rooms))
	}
	if rooms[0].OwnerUserID != ownerID {
		t.Errorf("expected owner_user_id %d, got %d", ownerID, rooms[0].OwnerUserID)
	}
}

func TestWatchRoom_ListForInvitedUser(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

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
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	outsider, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Outsider",
		Email:    "outsider@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

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
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	err = app.Queries.DeleteWatchRoom(ctx, room.ID)
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

func TestWatchRoom_IsOwner(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	_, err = app.Queries.IsWatchRoomOwner(ctx, database.IsWatchRoomOwnerParams{
		ID:          room.ID,
		OwnerUserID: ownerID,
	})
	if err != nil {
		t.Errorf("expected owner check to succeed, got: %v", err)
	}

	_, err = app.Queries.IsWatchRoomOwner(ctx, database.IsWatchRoomOwnerParams{
		ID:          room.ID,
		OwnerUserID: guest.ID,
	})
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for non-owner, got: %v", err)
	}
}

func TestWatchRoom_IsMember(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)

	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	outsider, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Outsider",
		Email:    "outsider@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	_, err = app.Queries.IsWatchRoomMember(ctx, database.IsWatchRoomMemberParams{
		RoomID: room.ID,
		UserID: guest.ID,
	})
	if err != nil {
		t.Errorf("expected guest to be a member, got: %v", err)
	}

	_, err = app.Queries.IsWatchRoomMember(ctx, database.IsWatchRoomMemberParams{
		RoomID: room.ID,
		UserID: outsider.ID,
	})
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for outsider, got: %v", err)
	}
}

func TestWatchRoom_CountUsersByIDs(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	u1, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "User One",
		Email:    "u1@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user1: %v", err)
	}
	u2, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "User Two",
		Email:    "u2@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user2: %v", err)
	}

	count, err := app.Queries.CountUsersByIDs(ctx, []int64{u1.ID, u2.ID})
	if err != nil {
		t.Fatalf("CountUsersByIDs failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	count, err = app.Queries.CountUsersByIDs(ctx, []int64{u1.ID, 99999})
	if err != nil {
		t.Fatalf("CountUsersByIDs with missing ID failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1 for one valid and one missing ID, got %d", count)
	}
}

func TestWatchRoom_CascadeDeleteMovie(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
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
	defer app.DB.Close()
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

type authenticatedWatchRoomRouter struct {
	app    *Application
	cookie *http.Cookie
}

func (h authenticatedWatchRoomRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.cookie != nil {
		r.AddCookie(h.cookie)
	}
	h.app.Router.ServeHTTP(w, r)
}

func mountWatchRoomRouter(t *testing.T, app *Application, userID int64) http.Handler {
	t.Helper()

	if app.Router == nil {
		app.InitRouter()
	}

	return authenticatedWatchRoomRouter{
		app:    app,
		cookie: newWatchRoomSessionCookie(t, app, userID),
	}
}

func performWatchRoomHTTPRequest(t *testing.T, app *Application, userID int64, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()

	if app.Router == nil {
		app.InitRouter()
	}

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID > 0 {
		req.AddCookie(newWatchRoomSessionCookie(t, app, userID))
	}

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":0,"invited_user_ids":[]}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error {
		t.Errorf("expected error=false, got error=true: %s", resp.Message)
	}
}

func TestCreateWatchRoom_HTTP_InvalidMode(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"direct","audio_track":-1}`, movieID)
	req := httptest.NewRequest(http.MethodPost, "/api/watch-rooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateWatchRoom_HTTP_NegativeSubtitleTrackRejected(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session cookie, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetWatchRooms_HTTP_ProductionRouterResponseShape(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
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

	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, "/api/watch-rooms", "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
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

	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID), "")
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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}
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

	owner, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     "HLS Owner",
		Email:    "hls-owner@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	movieID := insertTestHLSMovieFixture(t, app, "h264", 720)
	handler := mountWatchRoomRouter(t, app, owner.ID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	app.Settings = &database.Setting{}

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

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

func TestGetWatchRoom_HTTP_NotFound(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms/99999", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetWatchRoom_HTTP_ForbiddenForNonMember(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Outsider",
		Email:    "outsider@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := mountWatchRoomRouter(t, app, outsider.ID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member, got %d", w.Code)
	}
}

func TestGetWatchRoom_HTTP_SuccessForMember(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := mountWatchRoomRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJoinWatchRoom_HTTP_SuccessForMember(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := mountWatchRoomRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/watch-rooms/%d/join", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJoinWatchRoom_HTTP_ForbiddenForNonMember(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Outsider",
		Email:    "outsider@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := mountWatchRoomRouter(t, app, outsider.ID)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/watch-rooms/%d/join", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member join, got %d", w.Code)
	}
}

func TestDeleteWatchRoom_HTTP_SuccessForOwner(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := mountWatchRoomRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	_, err := app.Queries.GetWatchRoomByID(context.Background(), room.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected room to be deleted, got: %v", err)
	}
}

func TestDeleteWatchRoom_HTTP_CleansUpRoomHLSSession(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	handler := mountWatchRoomRouter(t, app, ownerID)

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
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	handler := mountWatchRoomRouter(t, app, guest.ID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/watch-rooms/%d", room.ID), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for invited non-owner delete, got %d", w.Code)
	}
}

func TestDeleteWatchRoom_HTTP_NotFound(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(t, app, ownerID)

	req := httptest.NewRequest(http.MethodDelete, "/api/watch-rooms/99999", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestStreamWatchRoomMovie_HTTP_DirectStreamUsesRealMembershipAndMode(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
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
	outsider, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Stream Outsider",
		Email:    "stream-outsider@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)

	path := fmt.Sprintf("/api/watch-rooms/%d/stream", room.ID)
	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, path, "")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for direct stream, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != body {
		t.Fatalf("stream body = %q, want %q", got, body)
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "video/mp4") {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}

	w = performWatchRoomHTTPRequest(t, app, outsider.ID, http.MethodGet, path, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member stream, got %d: %s", w.Code, w.Body.String())
	}

	hlsRoom := createTestRoomWithMode(t, app, ownerID, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	addMembersToRoom(t, app, hlsRoom.ID, ownerID)
	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/stream", hlsRoom.ID), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when streaming an HLS room directly, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWatchRoomHLS_HTTP_RequiresMembershipModeAndManifestBeforeSegments(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "HLS Outsider",
		Email:    "hls-outsider@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create outsider: %v", err)
	}

	directRoom := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, directRoom.ID, ownerID)
	w := performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/playlist.m3u8", directRoom.ID), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for HLS manifest on direct room, got %d: %s", w.Code, w.Body.String())
	}

	hlsRoom := createTestRoomWithMode(t, app, ownerID, movieID, helpers.HLS_PROFILE_720P_3MBPS)
	addMembersToRoom(t, app, hlsRoom.ID, ownerID)

	w = performWatchRoomHTTPRequest(t, app, outsider.ID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/segment_0.m4s", hlsRoom.ID), "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-member HLS segment, got %d: %s", w.Code, w.Body.String())
	}

	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/bad_name.m4s", hlsRoom.ID), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid segment request to be rejected, got %d: %s", w.Code, w.Body.String())
	}

	w = performWatchRoomHTTPRequest(t, app, ownerID, http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d/hls/segment_0.m4s", hlsRoom.ID), "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before manifest/session creation, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUsers_HTTP_ExcludesCurrentUser(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	user1, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user1: %v", err)
	}
	_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Bob",
		Email:    "bob@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user2: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user1.ID)
		app.GetUsers(w, r)
	})
	handler := app.SessionManager.LoadAndSave(r)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatal("expected data to be an object")
	}

	users, ok := data["users"].([]any)
	if !ok {
		t.Fatal("expected users array")
	}

	if len(users) != 1 {
		t.Errorf("expected 1 user in response (excluding self), got %d", len(users))
	}
}

func TestGetUsers_HTTP_SearchFilter(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	user1, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Alice Smith",
		Email:    "alice@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user1: %v", err)
	}
	_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Bob Jones",
		Email:    "bob@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user2: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user1.ID)
		app.GetUsers(w, r)
	})
	handler := app.SessionManager.LoadAndSave(r)

	req := httptest.NewRequest(http.MethodGet, "/api/users?q=bob", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	data := resp.Data.(map[string]any)
	users := data["users"].([]any)

	if len(users) != 1 {
		t.Errorf("expected 1 user matching 'bob', got %d", len(users))
	}
}

func TestGetUsers_HTTP_SearchTreatsLikeMetacharactersLiterally(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()
	ctx := context.Background()

	user1, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Alice Smith",
		Email:    "alice@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user1: %v", err)
	}
	_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Bob_One",
		Email:    "bob_one@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user2: %v", err)
	}
	_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "BobXOne",
		Email:    "bobxone@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user3: %v", err)
	}
	_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Carol%Two",
		Email:    "carol%two@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user4: %v", err)
	}
	_, err = app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "CarolTwo",
		Email:    "caroltwo@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create user5: %v", err)
	}

	r := chi.NewRouter()
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user1.ID)
		app.GetUsers(w, r)
	})
	handler := app.SessionManager.LoadAndSave(r)

	req := httptest.NewRequest(http.MethodGet, "/api/users?q=carol%25", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for percent search, got %d", w.Code)
	}

	var resp helpers.JSONResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode percent search response: %v", err)
	}

	data := resp.Data.(map[string]any)
	users := data["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user matching literal percent, got %d", len(users))
	}

	percentMatch := users[0].(map[string]any)
	if percentMatch["name"] != "Carol%Two" {
		t.Fatalf("expected Carol%%Two for percent search, got %#v", percentMatch["name"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users?q=bob_", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for underscore search, got %d", w.Code)
	}

	resp = helpers.JSONResponse{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode underscore search response: %v", err)
	}

	data = resp.Data.(map[string]any)
	users = data["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("expected 1 user matching literal underscore, got %d", len(users))
	}

	underscoreMatch := users[0].(map[string]any)
	if underscoreMatch["name"] != "Bob_One" {
		t.Fatalf("expected Bob_One for underscore search, got %#v", underscoreMatch["name"])
	}
}
