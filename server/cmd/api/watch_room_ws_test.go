package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

type watchRoomWSTestEvent struct {
	Type     string                  `json:"type"`
	RoomID   int64                   `json:"room_id"`
	Playback *watchRoomPlaybackState `json:"playback"`
}

func setupWatchRoomWSTestServer(t *testing.T, app *Application) *httptest.Server {
	t.Helper()

	app.InitSession()
	if app.Wait == nil {
		app.Wait = &sync.WaitGroup{}
	}

	router := chi.NewRouter()
	router.Get("/api/watch-rooms/{id}/ws", func(w http.ResponseWriter, r *http.Request) {
		userID, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
		if err == nil && userID > 0 {
			app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		}
		app.WatchRoomWebSocket(w, r)
	})

	return httptest.NewServer(app.SessionManager.LoadAndSave(router))
}

func dialWatchRoomSocket(t *testing.T, serverURL string, roomID, userID int64) (*websocket.Conn, *http.Response) {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + fmt.Sprintf("/api/watch-rooms/%d/ws?user_id=%d", roomID, userID)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			return nil, resp
		}
		t.Fatalf("dial websocket: %v", err)
	}
	return conn, resp
}

func readWatchRoomEvent(t *testing.T, conn *websocket.Conn) watchRoomWSTestEvent {
	t.Helper()

	err := conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	var event watchRoomWSTestEvent
	err = conn.ReadJSON(&event)
	if err != nil {
		t.Fatalf("read websocket event: %v", err)
	}
	return event
}

func readUntilEventType(t *testing.T, conn *websocket.Conn, eventType string) watchRoomWSTestEvent {
	t.Helper()

	for i := 0; i < 5; i++ {
		event := readWatchRoomEvent(t, conn)
		if event.Type == eventType {
			return event
		}
	}

	t.Fatalf("did not receive %q event", eventType)
	return watchRoomWSTestEvent{}
}

func readUntilPlaybackPosition(
	t *testing.T,
	conn *websocket.Conn,
	eventType string,
	minPosition float64,
	maxPosition float64,
) watchRoomWSTestEvent {
	t.Helper()

	for i := 0; i < 8; i++ {
		event := readWatchRoomEvent(t, conn)
		if event.Type != eventType || event.Playback == nil {
			continue
		}
		if event.Playback.PositionSec >= minPosition && event.Playback.PositionSec <= maxPosition {
			return event
		}
	}

	t.Fatalf("did not receive %q event with position in range %.3f-%.3f", eventType, minPosition, maxPosition)
	return watchRoomWSTestEvent{}
}

func expectNoEventType(t *testing.T, conn *websocket.Conn, forbiddenType string, wait time.Duration) {
	t.Helper()

	err := conn.SetReadDeadline(time.Now().Add(wait))
	if err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	for {
		var event watchRoomWSTestEvent
		err = conn.ReadJSON(&event)
		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				return
			}
			t.Fatalf("read websocket event: %v", err)
		}
		if event.Type == forbiddenType {
			t.Fatalf("unexpected %q event", forbiddenType)
		}
	}
}

func expectSocketToClose(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	err := conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	for {
		_, _, err = conn.ReadMessage()
		if err != nil {
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				t.Fatalf("expected websocket connection to close before read deadline, got timeout: %v", err)
			}
			return
		}
	}
}

func TestWatchRoomWebSocket_RejectsNonMember(t *testing.T) {
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

	room := createTestRoom(t, app, ownerID, movieID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	conn, resp := dialWatchRoomSocket(t, server.URL, room.ID, outsider.ID)
	if conn != nil {
		_ = conn.Close()
		t.Fatal("expected websocket dial to fail for non-member")
	}
	if resp == nil || resp.StatusCode != 403 {
		if resp == nil {
			t.Fatalf("expected 403 response for non-member websocket upgrade")
		}
		t.Fatalf("expected 403 response, got %d", resp.StatusCode)
	}
}

func TestWatchRoomWebSocket_BroadcastsPlaybackChanges(t *testing.T) {
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
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	ownerSnapshot := readUntilEventType(t, ownerConn, "room_snapshot")
	if ownerSnapshot.Playback == nil || !ownerSnapshot.Playback.Paused || ownerSnapshot.Playback.PositionSec != 0 {
		t.Fatalf("unexpected owner snapshot: %+v", ownerSnapshot.Playback)
	}

	guestSnapshot := readUntilEventType(t, guestConn, "room_snapshot")
	if guestSnapshot.Playback == nil || !guestSnapshot.Playback.Paused || guestSnapshot.Playback.PositionSec != 0 {
		t.Fatalf("unexpected guest snapshot: %+v", guestSnapshot.Playback)
	}

	err = ownerConn.WriteJSON(map[string]any{
		"type":         "play",
		"position_sec": 12.5,
	})
	if err != nil {
		t.Fatalf("owner send play event: %v", err)
	}

	playEvent := readUntilEventType(t, guestConn, "playback_changed")
	if playEvent.Playback == nil {
		t.Fatal("expected playback payload on play event")
	}
	if playEvent.Playback.Paused {
		t.Fatal("expected play event to set paused=false")
	}
	if playEvent.Playback.PositionSec < 12 || playEvent.Playback.PositionSec > 13 {
		t.Fatalf("expected play position near 12.5, got %.3f", playEvent.Playback.PositionSec)
	}

	err = guestConn.WriteJSON(map[string]any{
		"type":         "seek",
		"position_sec": 42.0,
	})
	if err != nil {
		t.Fatalf("guest send seek event: %v", err)
	}

	seekEvent := readUntilPlaybackPosition(t, ownerConn, "playback_changed", 41.5, 42.5)
	if seekEvent.Playback == nil {
		t.Fatal("expected playback payload on seek event")
	}

	err = ownerConn.WriteJSON(map[string]any{
		"type":         "pause",
		"position_sec": 45.0,
	})
	if err != nil {
		t.Fatalf("owner send pause event: %v", err)
	}

	pauseEvent := readUntilPlaybackPosition(t, guestConn, "playback_changed", 44.5, 45.5)
	if pauseEvent.Playback == nil {
		t.Fatal("expected playback payload on pause event")
	}
	if !pauseEvent.Playback.Paused {
		t.Fatal("expected pause event to set paused=true")
	}
}

func TestWatchRoomWebSocket_DoesNotBroadcastMemberJoinedForSecondSocket(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest-second-socket@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	_ = readUntilEventType(t, guestConn, "room_snapshot")

	ownerSecondConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, ownerID)
	defer ownerSecondConn.Close()

	_ = readUntilEventType(t, ownerSecondConn, "room_snapshot")
	expectNoEventType(t, guestConn, "member_joined", 250*time.Millisecond)
}

func TestWatchRoomWebSocket_ReceivesRoomDeletedOnDelete(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest-delete@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	_ = readUntilEventType(t, guestConn, "room_snapshot")

	err = app.Queries.DeleteWatchRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("delete watch room query failed: %v", err)
	}
	app.WatchRoomHub.deleteRoom(room.ID)

	ownerDeleted := readUntilEventType(t, ownerConn, "room_deleted")
	if ownerDeleted.RoomID != room.ID {
		t.Fatalf("owner room_deleted event had wrong room_id: got %d want %d", ownerDeleted.RoomID, room.ID)
	}

	guestDeleted := readUntilEventType(t, guestConn, "room_deleted")
	if guestDeleted.RoomID != room.ID {
		t.Fatalf("guest room_deleted event had wrong room_id: got %d want %d", guestDeleted.RoomID, room.ID)
	}
}

func TestWatchRoomHub_ShutdownClosesConnectionsAndClearsSessions(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest-shutdown@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	_ = readUntilEventType(t, guestConn, "room_snapshot")

	app.WatchRoomHub.Shutdown()

	expectSocketToClose(t, ownerConn)
	expectSocketToClose(t, guestConn)

	app.WatchRoomHub.mu.Lock()
	sessionCount := len(app.WatchRoomHub.sessions)
	app.WatchRoomHub.mu.Unlock()
	if sessionCount != 0 {
		t.Fatalf("expected hub sessions to be cleared after shutdown, got %d", sessionCount)
	}
}

func TestWatchRoomWebSocket_ShutdownReleasesWaitGroup(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	conn, _ := dialWatchRoomSocket(t, server.URL, room.ID, ownerID)
	defer conn.Close()

	_ = readUntilEventType(t, conn, "room_snapshot")

	done := make(chan struct{})
	go func() {
		app.Wait.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("expected wait group to remain blocked while websocket is connected")
	case <-time.After(150 * time.Millisecond):
	}

	app.WatchRoomHub.Shutdown()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected wait group to release after websocket shutdown")
	}
}

func TestWatchRoomHub_ShutdownIsIdempotent(t *testing.T) {
	hub := NewWatchRoomHub()
	hub.Shutdown()
	hub.Shutdown()
}
