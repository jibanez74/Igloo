package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

// createTestRoom creates a watch room owned by ownerID for the given movie,
// with an optional list of invited user IDs. Returns the created room.
func createTestRoom(t *testing.T, app *Application, ownerID, movieID int64, invitedIDs []int64) database.WatchRoom {
	t.Helper()
	ctx := context.Background()

	tx, err := app.DB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("createTestRoom: begin tx: %v", err)
	}

	qtx := app.Queries.WithTx(tx)

	room, err := qtx.CreateWatchRoom(ctx, database.CreateWatchRoomParams{
		OwnerUserID:  ownerID,
		MovieID:      movieID,
		PlaybackMode: "direct",
		AudioTrack:   0,
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("createTestRoom: create room: %v", err)
	}

	err = qtx.AddWatchRoomMember(ctx, database.AddWatchRoomMemberParams{
		RoomID: room.ID,
		UserID: ownerID,
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("createTestRoom: add owner member: %v", err)
	}

	for _, id := range invitedIDs {
		err = qtx.AddWatchRoomMember(ctx, database.AddWatchRoomMemberParams{
			RoomID: room.ID,
			UserID: id,
		})
		if err != nil {
			_ = tx.Rollback()
			t.Fatalf("createTestRoom: add invited member %d: %v", id, err)
		}
	}

	if err = tx.Commit(); err != nil {
		t.Fatalf("createTestRoom: commit: %v", err)
	}

	return room
}

func setupWatchRoomHTTPTestApp(t *testing.T) *Application {
	t.Helper()
	app := setupTestApp(t)
	app.InitSession()
	return app
}

// --- Query-level tests ---

func TestWatchRoom_CreateAndFetch(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)

	room := createTestRoom(t, app, ownerID, movieID, nil)

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
	app := setupTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID, nil)

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
	app := setupTestApp(t)
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

	room := createTestRoom(t, app, ownerID, movieID, []int64{guest1.ID, guest2.ID})

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
	room := createTestRoom(t, app, ownerID, movieID, nil)

	// Inserting the owner again should violate the UNIQUE (room_id, user_id) constraint.
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
	createTestRoom(t, app, ownerID, movieID, nil)

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

	createTestRoom(t, app, ownerID, movieID, []int64{guest.ID})

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

	createTestRoom(t, app, ownerID, movieID, nil)

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

	room := createTestRoom(t, app, ownerID, movieID, []int64{guest.ID})

	err = app.Queries.DeleteWatchRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("DeleteWatchRoom failed: %v", err)
	}

	_, err = app.Queries.GetWatchRoomByID(ctx, room.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected room to be deleted (sql.ErrNoRows), got: %v", err)
	}

	// Member rows must be cascade-deleted.
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

	room := createTestRoom(t, app, ownerID, movieID, []int64{guest.ID})

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

	room := createTestRoom(t, app, ownerID, movieID, []int64{guest.ID})

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
	room := createTestRoom(t, app, ownerID, movieID, nil)

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
	room := createTestRoom(t, app, ownerID, movieID, nil)

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

// --- HTTP handler tests ---

func mountWatchRoomRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/watch-rooms", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.GetWatchRooms(w, r)
	})
	r.Post("/api/watch-rooms", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.CreateWatchRoom(w, r)
	})
	r.Get("/api/watch-rooms/{id}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.GetWatchRoom(w, r)
	})
	r.Post("/api/watch-rooms/{id}/join", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.JoinWatchRoom(w, r)
	})
	r.Delete("/api/watch-rooms/{id}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.DeleteWatchRoom(w, r)
	})
	return app.SessionManager.LoadAndSave(r)
}

func TestCreateWatchRoom_HTTP_Success(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(app, ownerID)

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
	handler := mountWatchRoomRouter(app, ownerID)

	body := fmt.Sprintf(`{"movie_id":%d,"mode":"badmode","audio_track":0}`, movieID)
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
	handler := mountWatchRoomRouter(app, ownerID)

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
	handler := mountWatchRoomRouter(app, ownerID)

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
	handler := mountWatchRoomRouter(app, ownerID)

	// Include owner ID in invited list; it should be silently filtered.
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
	// Owner appears only once.
	if len(members) != 1 {
		t.Errorf("expected 1 member (owner once), got %d", len(members))
	}
}

func TestGetWatchRoom_HTTP_NotFound(t *testing.T) {
	app := setupWatchRoomHTTPTestApp(t)
	defer app.DB.Close()

	ownerID, _ := createTestUserAndMovie(t, app)
	handler := mountWatchRoomRouter(app, ownerID)

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

	room := createTestRoom(t, app, ownerID, movieID, nil)
	handler := mountWatchRoomRouter(app, outsider.ID)

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
	room := createTestRoom(t, app, ownerID, movieID, nil)
	handler := mountWatchRoomRouter(app, ownerID)

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
	room := createTestRoom(t, app, ownerID, movieID, nil)
	handler := mountWatchRoomRouter(app, ownerID)

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

	room := createTestRoom(t, app, ownerID, movieID, nil)
	handler := mountWatchRoomRouter(app, outsider.ID)

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
	room := createTestRoom(t, app, ownerID, movieID, nil)
	handler := mountWatchRoomRouter(app, ownerID)

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
	room := createTestRoom(t, app, ownerID, movieID, nil)
	handler := mountWatchRoomRouter(app, ownerID)

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

	room := createTestRoom(t, app, ownerID, movieID, []int64{guest.ID})
	// Guest tries to delete the room.
	handler := mountWatchRoomRouter(app, guest.ID)

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
	handler := mountWatchRoomRouter(app, ownerID)

	req := httptest.NewRequest(http.MethodDelete, "/api/watch-rooms/99999", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
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

	// user1 made the request, so only Bob should appear.
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
