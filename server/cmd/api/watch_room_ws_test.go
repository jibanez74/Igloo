package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/gorilla/websocket"
)

type watchRoomWSTestEvent struct {
	Type             string                  `json:"type"`
	RoomID           int64                   `json:"room_id"`
	Playback         *watchRoomPlaybackState `json:"playback"`
	Member           *watchRoomMemberSummary `json:"member"`
	ConnectedUserIDs []int64                 `json:"connected_user_ids"`
}

type countingSessionStore struct {
	store interface {
		Delete(string) error
		Find(string) ([]byte, bool, error)
		Commit(string, []byte, time.Time) error
	}
	mu      sync.Mutex
	commits int
}

func (s *countingSessionStore) Delete(token string) error {
	return s.store.Delete(token)
}

func (s *countingSessionStore) Find(token string) ([]byte, bool, error) {
	return s.store.Find(token)
}

func (s *countingSessionStore) Commit(token string, b []byte, expiry time.Time) error {
	s.mu.Lock()
	s.commits++
	s.mu.Unlock()
	return s.store.Commit(token, b, expiry)
}

func (s *countingSessionStore) DeleteCtx(ctx context.Context, token string) error {
	if store, ok := s.store.(interface {
		DeleteCtx(context.Context, string) error
	}); ok {
		return store.DeleteCtx(ctx, token)
	}
	return s.Delete(token)
}

func (s *countingSessionStore) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	if store, ok := s.store.(interface {
		FindCtx(context.Context, string) ([]byte, bool, error)
	}); ok {
		return store.FindCtx(ctx, token)
	}
	return s.Find(token)
}

func (s *countingSessionStore) CommitCtx(ctx context.Context, token string, b []byte, expiry time.Time) error {
	s.mu.Lock()
	s.commits++
	s.mu.Unlock()
	if store, ok := s.store.(interface {
		CommitCtx(context.Context, string, []byte, time.Time) error
	}); ok {
		return store.CommitCtx(ctx, token, b, expiry)
	}
	return s.store.Commit(token, b, expiry)
}

func (s *countingSessionStore) CommitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commits
}

func setupWatchRoomWSTestServer(t *testing.T, app *Application) *httptest.Server {
	t.Helper()

	app.InitSession()
	if app.Wait == nil {
		app.Wait = &sync.WaitGroup{}
	}

	app.InitRouter()

	return httptest.NewServer(app.Router)
}

func closeWatchRoomWSTestApp(t *testing.T, app *Application) {
	t.Helper()

	if app.WatchRoomHub != nil {
		app.WatchRoomHub.Shutdown()
	}
	if app.Wait != nil {
		app.Wait.Wait()
	}
	if app.DB != nil {
		err := app.DB.Close()
		if err != nil {
			t.Fatalf("close test database: %v", err)
		}
	}
}

func newWatchRoomSessionCookie(t *testing.T, app *Application, userID int64) *http.Cookie {
	t.Helper()

	ctx, err := app.SessionManager.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("load test session: %v", err)
	}

	app.SessionManager.Put(ctx, helpers.COOKIE_USER_ID, userID)
	token, _, err := app.SessionManager.Commit(ctx)
	if err != nil {
		t.Fatalf("commit test session: %v", err)
	}

	return &http.Cookie{
		Name:  app.SessionManager.Cookie.Name,
		Value: token,
	}
}

func dialWatchRoomSocketWithCookie(
	t *testing.T,
	serverURL string,
	roomID int64,
	cookie *http.Cookie,
) (*websocket.Conn, *http.Response) {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + fmt.Sprintf("/api/watch-rooms/%d/ws", roomID)
	header := http.Header{}
	header.Add("Cookie", cookie.String())
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			return nil, resp
		}
		t.Fatalf("dial websocket: %v", err)
	}
	return conn, resp
}

func dialWatchRoomSocket(t *testing.T, app *Application, serverURL string, roomID, userID int64) (*websocket.Conn, *http.Response) {
	t.Helper()

	return dialWatchRoomSocketWithCookie(t, serverURL, roomID, newWatchRoomSessionCookie(t, app, userID))
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

func expectConnectedUserIDs(t *testing.T, event watchRoomWSTestEvent, want ...int64) {
	t.Helper()

	if len(event.ConnectedUserIDs) != len(want) {
		t.Fatalf("connected_user_ids = %v, want %v", event.ConnectedUserIDs, want)
	}

	got := make(map[int64]int, len(event.ConnectedUserIDs))
	for _, userID := range event.ConnectedUserIDs {
		got[userID]++
	}
	for _, userID := range want {
		if got[userID] == 0 {
			t.Fatalf("connected_user_ids = %v, want %v", event.ConnectedUserIDs, want)
		}
		got[userID]--
	}
}

func TestWatchRoomWebSocket_RejectsNonMember(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

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

	conn, resp := dialWatchRoomSocket(t, app, server.URL, room.ID, outsider.ID)
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

func TestWatchRoomWebSocket_LoadsSessionReadOnlyWithoutCommit(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	cookie := newWatchRoomSessionCookie(t, app, ownerID)
	store := &countingSessionStore{store: app.SessionManager.Store}
	app.SessionManager.Store = store

	conn, resp := dialWatchRoomSocketWithCookie(t, server.URL, room.ID, cookie)
	if conn == nil {
		if resp != nil {
			t.Fatalf("expected websocket connection, got status %d", resp.StatusCode)
		}
		t.Fatal("expected websocket connection")
	}

	snapshot := readUntilEventType(t, conn, "room_snapshot")
	if snapshot.RoomID != room.ID {
		t.Fatalf("snapshot room_id = %d, want %d", snapshot.RoomID, room.ID)
	}

	if setCookies := resp.Header.Values("Set-Cookie"); len(setCookies) != 0 {
		t.Fatalf("websocket handshake wrote session cookies: %v", setCookies)
	}

	_ = conn.Close()
	app.WatchRoomHub.Shutdown()
	app.Wait.Wait()

	if commits := store.CommitCount(); commits != 0 {
		t.Fatalf("read-only websocket session made %d store commits, want 0", commits)
	}
}

func TestWatchRoomWebSocket_PingPongAndIgnoresMalformedMessages(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer conn.Close()

	_ = readUntilEventType(t, conn, "room_snapshot")

	if err := conn.WriteMessage(websocket.TextMessage, []byte("{not json")); err != nil {
		t.Fatalf("write malformed message: %v", err)
	}
	if err := conn.WriteJSON(map[string]any{"type": "unknown"}); err != nil {
		t.Fatalf("write unknown message: %v", err)
	}
	if err := conn.WriteJSON(map[string]any{"type": "ping"}); err != nil {
		t.Fatalf("write ping message: %v", err)
	}

	pong := readUntilEventType(t, conn, "pong")
	if pong.RoomID != room.ID {
		t.Fatalf("pong room_id = %d, want %d", pong.RoomID, room.ID)
	}
}

func TestIsAllowedWatchRoomOrigin(t *testing.T) {
	tests := []struct {
		name          string
		requestURL    string
		host          string
		origin        string
		viteDevServer string
		tls           bool
		want          bool
	}{
		{
			name:       "allows same origin request",
			requestURL: "http://localhost:8080/api/watch-rooms/2/ws",
			host:       "localhost:8080",
			origin:     "http://localhost:8080",
			want:       true,
		},
		{
			name:       "rejects unrelated origin by default",
			requestURL: "http://localhost:8080/api/watch-rooms/2/ws",
			host:       "localhost:8080",
			origin:     "http://evil.example",
			want:       false,
		},
		{
			name:          "allows configured vite dev origin",
			requestURL:    "http://localhost:8080/api/watch-rooms/2/ws",
			host:          "localhost:8080",
			origin:        "http://localhost:3000",
			viteDevServer: "http://localhost:3000",
			want:          true,
		},
		{
			name:          "requires exact configured vite dev origin",
			requestURL:    "http://localhost:8080/api/watch-rooms/2/ws",
			host:          "localhost:8080",
			origin:        "http://127.0.0.1:3000",
			viteDevServer: "http://localhost:3000",
			want:          false,
		},
		{
			name:       "allows local dev origin without vite env",
			requestURL: "http://localhost:8080/api/watch-rooms/2/ws",
			host:       "localhost:8080",
			origin:     "http://localhost:3000",
			want:       true,
		},
		{
			name:       "allows same origin https request",
			requestURL: "https://igloo.tailnet/api/watch-rooms/2/ws",
			host:       "igloo.tailnet",
			origin:     "https://igloo.tailnet",
			tls:        true,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestURL, nil)
			req.Host = tt.host
			req.Header.Set("Origin", tt.origin)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			t.Setenv("VITE_DEV_SERVER", tt.viteDevServer)

			got := isAllowedWatchRoomOrigin(req)
			if got != tt.want {
				t.Fatalf("isAllowedWatchRoomOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWatchRoomWebSocket_BroadcastsPresenceAndMemberLeft(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Presence Guest",
		Email:    "presence-guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	ownerSnapshot := readUntilEventType(t, ownerConn, "room_snapshot")
	expectConnectedUserIDs(t, ownerSnapshot, ownerID)

	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	guestSnapshot := readUntilEventType(t, guestConn, "room_snapshot")
	expectConnectedUserIDs(t, guestSnapshot, ownerID, guest.ID)

	joined := readUntilEventType(t, ownerConn, "member_joined")
	if joined.Member == nil || joined.Member.ID != guest.ID {
		t.Fatalf("member_joined member = %+v, want guest %d", joined.Member, guest.ID)
	}
	expectConnectedUserIDs(t, joined, ownerID, guest.ID)

	_ = guestConn.Close()
	left := readUntilEventType(t, ownerConn, "member_left")
	if left.Member == nil || left.Member.ID != guest.ID {
		t.Fatalf("member_left member = %+v, want guest %d", left.Member, guest.ID)
	}
	expectConnectedUserIDs(t, left, ownerID)
}

func TestWatchRoomWebSocket_BroadcastsPlaybackChanges(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

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

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
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

func TestWatchRoomWebSocket_PlaybackEventsUseCurrentPositionWhenMissing(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Fallback Guest",
		Email:    "fallback-guest@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	_ = readUntilEventType(t, guestConn, "room_snapshot")

	if err := ownerConn.WriteJSON(map[string]any{
		"type":         "play",
		"position_sec": 5,
	}); err != nil {
		t.Fatalf("owner send play event: %v", err)
	}
	_ = readUntilEventType(t, guestConn, "playback_changed")

	time.Sleep(100 * time.Millisecond)

	if err := ownerConn.WriteJSON(map[string]any{"type": "pause"}); err != nil {
		t.Fatalf("owner send pause without position: %v", err)
	}

	pauseEvent := readUntilEventType(t, guestConn, "playback_changed")
	if pauseEvent.Playback == nil {
		t.Fatal("expected playback payload on pause event")
	}
	if !pauseEvent.Playback.Paused {
		t.Fatalf("expected pause fallback event to set paused=true, got %+v", pauseEvent.Playback)
	}
	if pauseEvent.Playback.PositionSec <= 5 {
		t.Fatalf("expected fallback pause position to advance beyond 5s, got %.3f", pauseEvent.Playback.PositionSec)
	}
}

func TestWatchRoomWebSocket_JoinSnapshotReflectsCurrentPlaybackState(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest-join-snapshot@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	_ = readUntilEventType(t, guestConn, "room_snapshot")

	err = ownerConn.WriteJSON(map[string]any{
		"type":         "play",
		"position_sec": 18.5,
	})
	if err != nil {
		t.Fatalf("owner send play event: %v", err)
	}

	playEvent := readUntilPlaybackPosition(t, guestConn, "playback_changed", 18.0, 19.5)
	if playEvent.Playback == nil || playEvent.Playback.Paused {
		t.Fatalf("expected guest play event to reflect active playback, got %+v", playEvent.Playback)
	}

	err = guestConn.WriteJSON(map[string]any{
		"type": "join",
	})
	if err != nil {
		t.Fatalf("guest send join event: %v", err)
	}

	joinSnapshot := readUntilEventType(t, guestConn, "room_snapshot")
	if joinSnapshot.Playback == nil {
		t.Fatal("expected room snapshot playback on join")
	}
	if joinSnapshot.Playback.Paused {
		t.Fatalf("expected join snapshot to keep playback running, got %+v", joinSnapshot.Playback)
	}
	if joinSnapshot.Playback.PositionSec < 18.0 || joinSnapshot.Playback.PositionSec > 20.0 {
		t.Fatalf("expected join snapshot position near active playback, got %.3f", joinSnapshot.Playback.PositionSec)
	}
}

func TestWatchRoomWebSocket_DoesNotBroadcastMemberJoinedForSecondSocket(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

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

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	defer guestConn.Close()

	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	_ = readUntilEventType(t, guestConn, "room_snapshot")

	ownerSecondConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerSecondConn.Close()

	_ = readUntilEventType(t, ownerSecondConn, "room_snapshot")
	expectNoEventType(t, guestConn, "member_joined", 250*time.Millisecond)
}

func TestWatchRoomWebSocket_DoesNotBroadcastMemberLeftUntilLastSocketCloses(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Guest",
		Email:    "guest-last-socket@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	observer, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Observer",
		Email:    "observer-last-socket@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create observer: %v", err)
	}

	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID, observer.ID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	defer guestConn.Close()
	_ = readUntilEventType(t, guestConn, "room_snapshot")
	observerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, observer.ID)
	defer observerConn.Close()
	_ = readUntilEventType(t, observerConn, "room_snapshot")
	observerJoined := readUntilEventType(t, guestConn, "member_joined")
	if observerJoined.Member == nil || observerJoined.Member.ID != observer.ID {
		t.Fatalf("member_joined member = %+v, want observer %d", observerJoined.Member, observer.ID)
	}

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	_ = readUntilEventType(t, ownerConn, "room_snapshot")
	joined := readUntilEventType(t, guestConn, "member_joined")
	if joined.Member == nil || joined.Member.ID != ownerID {
		t.Fatalf("member_joined member = %+v, want owner %d", joined.Member, ownerID)
	}
	expectConnectedUserIDs(t, joined, guest.ID, observer.ID, ownerID)
	_ = readUntilEventType(t, observerConn, "member_joined")

	ownerSecondConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerSecondConn.Close()
	_ = readUntilEventType(t, ownerSecondConn, "room_snapshot")

	_ = ownerSecondConn.Close()
	expectNoEventType(t, guestConn, "member_left", 250*time.Millisecond)

	_ = ownerConn.Close()
	left := readUntilEventType(t, observerConn, "member_left")
	if left.Member == nil || left.Member.ID != ownerID {
		t.Fatalf("member_left member = %+v, want owner %d", left.Member, ownerID)
	}
	expectConnectedUserIDs(t, left, guest.ID, observer.ID)
}

func TestWatchRoomWebSocket_ReceivesRoomDeletedOnDelete(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

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

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
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
	defer closeWatchRoomWSTestApp(t, app)

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

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
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
	defer closeWatchRoomWSTestApp(t, app)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
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

func TestWatchRoomClient_EnqueueEvictsStalledClientWithoutBlocking(t *testing.T) {
	client := newWatchRoomClient(nil, 1, watchRoomMemberSummary{ID: 1})

	for i := 0; i < watchRoomSendBufferSize; i++ {
		client.send <- []byte("queued")
	}

	done := make(chan struct{})
	go func() {
		client.enqueue([]byte("overflow"))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("enqueue blocked on a full outbox")
	}

	select {
	case <-client.done:
	default:
		t.Fatal("expected stalled client to be marked closed when its outbox is full")
	}
}

func TestWatchRoomWebSocket_ServerPingKeepsIdleConnectionAlive(t *testing.T) {
	origReadTimeout := watchRoomReadTimeout
	origPingInterval := watchRoomPingInterval
	watchRoomReadTimeout = 250 * time.Millisecond
	watchRoomPingInterval = 100 * time.Millisecond
	defer func() {
		watchRoomReadTimeout = origReadTimeout
		watchRoomPingInterval = origPingInterval
	}()

	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer conn.Close()

	_ = readUntilEventType(t, conn, "room_snapshot")

	// Block in a read for several server read-timeout windows. Gorilla's
	// default ping handler answers the server's ping control frames while
	// this goroutine is blocked reading, which must keep the connection
	// alive; a server-side disconnect would surface as a read error.
	type readResult struct {
		event watchRoomWSTestEvent
		err   error
	}
	results := make(chan readResult, 1)
	go func() {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var event watchRoomWSTestEvent
		err := conn.ReadJSON(&event)
		results <- readResult{event: event, err: err}
	}()

	time.Sleep(time.Second)

	if err := conn.WriteJSON(map[string]any{"type": "ping"}); err != nil {
		t.Fatalf("write ping after idle period: %v", err)
	}

	select {
	case res := <-results:
		if res.err != nil {
			t.Fatalf("connection dropped while idle: %v", res.err)
		}
		if res.event.Type != "pong" || res.event.RoomID != room.ID {
			t.Fatalf("unexpected event after idle period: %+v", res.event)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for pong after idle period")
	}
}
