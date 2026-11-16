package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"

	"github.com/gorilla/websocket"
)

func dialWatchRoomSocketWithBearer(t *testing.T, serverURL string, roomID int64, token string) (*websocket.Conn, *http.Response) {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + fmt.Sprintf("/api/watch-rooms/%d/ws", roomID)
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			return nil, resp
		}
		t.Fatalf("dial websocket: %v", err)
	}
	return conn, resp
}

// readUntilEventTypes reads until it has seen each type once, in any order.
func readUntilEventTypes(t *testing.T, conn *websocket.Conn, types ...string) map[string]watchRoomWSTestEvent {
	t.Helper()

	seen := make(map[string]watchRoomWSTestEvent, len(types))
	for i := 0; i < 10 && len(seen) < len(types); i++ {
		event := readWatchRoomEvent(t, conn)
		if slices.Contains(types, event.Type) {
			seen[event.Type] = event
		}
	}
	if len(seen) < len(types) {
		t.Fatalf("saw %v, want all of %v", seen, types)
	}
	return seen
}

func watchRoomMemberIDs(t *testing.T, app *Application, roomID int64) []int64 {
	t.Helper()

	ids, err := app.Queries.ListWatchRoomMemberIDs(context.Background(), roomID)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	return ids
}

func summaryIDs(members []userSummary) []int64 {
	ids := make([]int64, len(members))
	for i, member := range members {
		ids[i] = member.ID
	}
	return ids
}

func isWatchRoomAuthCached(app *Application, roomID, userID int64) bool {
	app.WatchRoomAuthCache.mu.Lock()
	defer app.WatchRoomAuthCache.mu.Unlock()
	_, cached := app.WatchRoomAuthCache.entries[watchRoomAuthKey{roomID: roomID, userID: userID}]
	return cached
}

func membersRequest(roomID int64, body string) *http.Request {
	return newOpenAPIJSONRequest(http.MethodPut, fmt.Sprintf("/api/watch-rooms/%d/members", roomID), body)
}

func leaveRequest(roomID int64) *http.Request {
	return newOpenAPIJSONRequest(http.MethodPost, fmt.Sprintf("/api/watch-rooms/%d/leave", roomID), "")
}

// The owner swaps one invitee for another: the removed guest's socket hears
// room_access_revoked and closes, the owner hears the new member list and the
// guest leaving, and the guest's cached access is gone.
func TestUpdateWatchRoomMembers_ReplacesInviteesAndRevokesRemoved(t *testing.T) {
	app := setupTestApp(t)
	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "members-guest@example.com", false)
	added := createTestUser(t, app, "Added", "members-added@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)

	_, err := app.loadAuthorizedWatchRoom(ctx, room.ID, guest.ID)
	if err != nil {
		t.Fatal(err)
	}

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	readUntilEventType(t, ownerConn, "room_snapshot")
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	defer guestConn.Close()
	readUntilEventType(t, guestConn, "room_snapshot")
	readUntilEventType(t, ownerConn, "member_joined")

	ownerCookie := newAuthSessionCookie(t, app, ownerID)
	body := fmt.Sprintf(`{"invited_user_ids":[%d,%d,%d]}`, added.ID, added.ID, ownerID)
	req := membersRequest(room.ID, body)
	req.AddCookie(ownerCookie)
	w := serveOpenAPIExchange(t, app.Router, "updateWatchRoomMembers", req, http.StatusOK)

	var envelope struct {
		Data struct {
			Room watchRoomDetail `json:"room"`
		} `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &envelope)
	if err != nil {
		t.Fatal(err)
	}
	if got := summaryIDs(envelope.Data.Room.Members); !slices.Equal(got, []int64{ownerID, added.ID}) {
		t.Fatalf("response members = %v, want owner then added", got)
	}
	if got := watchRoomMemberIDs(t, app, room.ID); !slices.Equal(got, []int64{ownerID, added.ID}) {
		t.Fatalf("stored members = %v", got)
	}
	if isWatchRoomAuthCached(app, room.ID, guest.ID) {
		t.Fatal("the removed guest's cached access survived")
	}

	revoked := readUntilEventType(t, guestConn, "room_access_revoked")
	if revoked.RoomID != room.ID {
		t.Fatalf("room_access_revoked room_id = %d", revoked.RoomID)
	}
	expectSocketToClose(t, guestConn)

	// The two come from different goroutines, in either order.
	events := readUntilEventTypes(t, ownerConn, "members_changed", "member_left")
	changed, left := events["members_changed"], events["member_left"]
	if got := summaryIDs(changed.Members); !slices.Equal(got, []int64{ownerID, added.ID}) {
		t.Fatalf("members_changed members = %v", got)
	}
	if left.Member == nil || left.Member.ID != guest.ID {
		t.Fatalf("member_left = %+v, want the removed guest", left.Member)
	}

	// The removed guest is refused from now on; the added user is let in.
	conn, resp := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	if conn != nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("removed guest reconnect status = %v", resp)
	}
	addedConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, added.ID)
	defer addedConn.Close()
	readUntilEventType(t, addedConn, "room_snapshot")
}

func TestUpdateWatchRoomMembers_RefusesBadRequests(t *testing.T) {
	app := setupSessionTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "members-refused@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

	cases := []struct {
		name   string
		caller int64
		req    *http.Request
		want   int
	}{
		{"guest is not the owner", guest.ID, membersRequest(room.ID, `{"invited_user_ids":[]}`), http.StatusForbidden},
		{"unknown room", ownerID, membersRequest(room.ID+100, `{"invited_user_ids":[]}`), http.StatusNotFound},
		{"unknown user", ownerID, membersRequest(room.ID, `{"invited_user_ids":[999999]}`), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serveOpenAPIExchange(t, authenticatedRouter(t, app, tc.caller), "updateWatchRoomMembers", tc.req, tc.want)
		})
	}

	// Bodies the contract itself rejects, so the exchange is not validated.
	target := fmt.Sprintf("/api/watch-rooms/%d/members", room.ID)
	for _, body := range []string{`{}`, `{"invited_user_ids":[],"owner":1}`, `{"invited_user_ids":null}`} {
		w := serveRequest(t, authenticatedRouter(t, app, ownerID), http.MethodPut, target, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, w.Code)
		}
	}

	if got := watchRoomMemberIDs(t, app, room.ID); !slices.Equal(got, []int64{ownerID, guest.ID}) {
		t.Fatalf("a refused request changed the members: %v", got)
	}

	// An empty list leaves the owner alone in the room.
	serveOpenAPIExchange(t, authenticatedRouter(t, app, ownerID), "updateWatchRoomMembers",
		membersRequest(room.ID, `{"invited_user_ids":[]}`), http.StatusOK)
	if got := watchRoomMemberIDs(t, app, room.ID); !slices.Equal(got, []int64{ownerID}) {
		t.Fatalf("members after clearing = %v, want only the owner", got)
	}
}

// A guest who leaves is removed on every device: both of their sockets hear
// room_access_revoked, and the owner hears the shorter member list.
func TestLeaveWatchRoom_RemovesCallerEverywhere(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "leave-guest@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	readUntilEventType(t, ownerConn, "room_snapshot")
	guestCookie := newAuthSessionCookie(t, app, guest.ID)
	guestBrowser, _ := dialWatchRoomSocketWithCookie(t, server.URL, room.ID, guestCookie)
	defer guestBrowser.Close()
	readUntilEventType(t, guestBrowser, "room_snapshot")
	guestTV, _ := dialWatchRoomSocketWithBearer(t, server.URL, room.ID, createTestDevice(t, app, guest.ID, "TV", "android_tv"))
	defer guestTV.Close()
	readUntilEventType(t, guestTV, "room_snapshot")

	req := leaveRequest(room.ID)
	req.AddCookie(guestCookie)
	w := serveOpenAPIExchange(t, app.Router, "leaveWatchRoom", req, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"left":true`) {
		t.Fatalf("leave body = %s", w.Body.String())
	}

	for _, conn := range []*websocket.Conn{guestBrowser, guestTV} {
		readUntilEventType(t, conn, "room_access_revoked")
		expectSocketToClose(t, conn)
	}
	changed := readUntilEventType(t, ownerConn, "members_changed")
	if got := summaryIDs(changed.Members); !slices.Equal(got, []int64{ownerID}) {
		t.Fatalf("members_changed members = %v", got)
	}
	if got := watchRoomMemberIDs(t, app, room.ID); !slices.Equal(got, []int64{ownerID}) {
		t.Fatalf("stored members = %v", got)
	}
}

func TestLeaveWatchRoom_RefusesOwnerAndOutsiders(t *testing.T) {
	app := setupSessionTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	outsider := createTestUser(t, app, "Outsider", "leave-outsider@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)

	serveOpenAPIExchange(t, authenticatedRouter(t, app, ownerID), "leaveWatchRoom", leaveRequest(room.ID), http.StatusConflict)
	serveOpenAPIExchange(t, authenticatedRouter(t, app, outsider.ID), "leaveWatchRoom", leaveRequest(room.ID), http.StatusForbidden)
	serveOpenAPIExchange(t, authenticatedRouter(t, app, ownerID), "leaveWatchRoom", leaveRequest(room.ID+100), http.StatusNotFound)

	if got := watchRoomMemberIDs(t, app, room.ID); !slices.Equal(got, []int64{ownerID}) {
		t.Fatalf("members = %v, want the owner kept", got)
	}
}

func TestJoinWatchRoom_HTTP_UnknownRoomIsNotFound(t *testing.T) {
	app := setupSessionTestApp(t)
	ownerID, _ := createTestUserAndMovie(t, app)

	req := newOpenAPIJSONRequest(http.MethodPost, "/api/watch-rooms/4242/join", "")
	serveOpenAPIExchange(t, authenticatedRouter(t, app, ownerID), "joinWatchRoom", req, http.StatusNotFound)
}

// Deleting an invited member's account tells the rest of the room the new
// member list, and closes the member's own socket with their credentials.
func TestDeletingInvitedMemberAnnouncesMembers(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "deleted-member@example.com", false)
	admin := createTestUser(t, app, "Admin", "deleted-member-admin@example.com", true)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)

	ownerConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer ownerConn.Close()
	readUntilEventType(t, ownerConn, "room_snapshot")
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	defer guestConn.Close()
	readUntilEventType(t, guestConn, "room_snapshot")
	readUntilEventType(t, ownerConn, "member_joined")

	req := newOpenAPIJSONRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(guest.ID, 10), "")
	serveOpenAPIExchange(t, authenticatedRouter(t, app, admin.ID), "adminDeleteUser", req, http.StatusOK)

	changed := readUntilEventType(t, ownerConn, "members_changed")
	if got := summaryIDs(changed.Members); !slices.Equal(got, []int64{ownerID}) {
		t.Fatalf("members_changed members = %v", got)
	}
	expectSocketToClose(t, guestConn)
}

// Each way of ending a credential closes the sockets it opened, and only
// those: the room stays open on the user's other credential.
func TestWatchRoomWebSocket_ClosedWhenCredentialRevoked(t *testing.T) {
	cases := []struct {
		name string
		// revoke ends the credential under test; closes names the socket it
		// must close, and the other must stay open.
		revoke func(t *testing.T, app *Application, userID int64, cookie *http.Cookie, deviceID int64) *httptest.ResponseRecorder
		closes string
	}{
		{
			name: "cookie logout",
			revoke: func(t *testing.T, app *Application, _ int64, cookie *http.Cookie, _ int64) *httptest.ResponseRecorder {
				return serveWithCookie(t, app, cookie, http.MethodDelete, "/api/auth/logout", "")
			},
			closes: "cookie",
		},
		{
			name: "device revoked",
			revoke: func(t *testing.T, app *Application, userID int64, _ *http.Cookie, deviceID int64) *httptest.ResponseRecorder {
				// Revoke from another browser session, as Settings does.
				other := newAuthSessionCookie(t, app, userID)
				return serveWithCookie(t, app, other, http.MethodDelete, fmt.Sprintf("/api/devices/%d", deviceID), "")
			},
			closes: "device",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupTestApp(t)
			ownerID, movieID := createTestUserAndMovie(t, app)
			room := createTestRoom(t, app, ownerID, movieID)
			addMembersToRoom(t, app, room.ID, ownerID)
			server := setupWatchRoomWSTestServer(t, app)

			cookie := newAuthSessionCookie(t, app, ownerID)
			cookieConn, _ := dialWatchRoomSocketWithCookie(t, server.URL, room.ID, cookie)
			defer cookieConn.Close()
			readUntilEventType(t, cookieConn, "room_snapshot")

			token := createTestDevice(t, app, ownerID, "TV", "android_tv")
			devices, err := app.Queries.GetDevicesByUser(context.Background(), ownerID)
			if err != nil || len(devices) != 1 {
				t.Fatalf("devices = %v, %v", devices, err)
			}
			deviceConn, _ := dialWatchRoomSocketWithBearer(t, server.URL, room.ID, token)
			defer deviceConn.Close()
			readUntilEventType(t, deviceConn, "room_snapshot")

			w := tc.revoke(t, app, ownerID, cookie, devices[0].ID)
			if w.Code != http.StatusOK {
				t.Fatalf("revoke status = %d: %s", w.Code, w.Body.String())
			}

			closed, kept := cookieConn, deviceConn
			if tc.closes == "device" {
				closed, kept = deviceConn, cookieConn
			}
			expectSocketToClose(t, closed)
			// The other socket still answers, and was never told to leave.
			err = kept.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`))
			if err != nil {
				t.Fatalf("kept socket write: %v", err)
			}
			for {
				event := readWatchRoomEvent(t, kept)
				if event.Type == "room_access_revoked" {
					t.Fatal("the kept socket was revoked")
				}
				if event.Type == "pong" {
					break
				}
			}
		})
	}
}

// A password change ends every other session and every device: their sockets
// close, while the session that changed it carries on under a renewed token.
func TestWatchRoomWebSocket_PasswordChangeClosesOtherSessions(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)

	otherConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer otherConn.Close()
	readUntilEventType(t, otherConn, "room_snapshot")
	deviceConn, _ := dialWatchRoomSocketWithBearer(t, server.URL, room.ID, createTestDevice(t, app, ownerID, "TV", "android_tv"))
	defer deviceConn.Close()
	readUntilEventType(t, deviceConn, "room_snapshot")

	body := fmt.Sprintf(`{"current_password":%q,"new_password":"a new correct horse"}`, testUserPassword)
	w := serveWithCookie(t, app, newAuthSessionCookie(t, app, ownerID), http.MethodPut, "/api/user/password", body)
	if w.Code != http.StatusOK {
		t.Fatalf("password change status = %d: %s", w.Code, w.Body.String())
	}

	expectSocketToClose(t, otherConn)
	expectSocketToClose(t, deviceConn)
	conn, resp := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	if conn == nil {
		t.Fatalf("a fresh session could not reconnect: %v", resp)
	}
	conn.Close()
}

// A device token opens the socket and the room routes, as the TV uses them.
func TestWatchRoom_DeviceTokenReachesRoutesAndSocket(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "TV Guest", "tv-guest@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)
	token := createTestDevice(t, app, guest.ID, "TV", "android_tv")

	for _, tc := range []struct {
		operationID string
		req         *http.Request
	}{
		{"getWatchRooms", newOpenAPIJSONRequest(http.MethodGet, "/api/watch-rooms", "")},
		{"getWatchRoom", newOpenAPIJSONRequest(http.MethodGet, fmt.Sprintf("/api/watch-rooms/%d", room.ID), "")},
		{"joinWatchRoom", newOpenAPIJSONRequest(http.MethodPost, fmt.Sprintf("/api/watch-rooms/%d/join", room.ID), "")},
	} {
		tc.req.Header.Set("Authorization", "Bearer "+token)
		serveOpenAPIExchange(t, app.Router, tc.operationID, tc.req, http.StatusOK)
	}

	conn, resp := dialWatchRoomSocketWithBearer(t, server.URL, room.ID, token)
	if conn == nil {
		t.Fatalf("bearer upgrade failed: %v", resp)
	}
	defer conn.Close()
	snapshot := readUntilEventType(t, conn, "room_snapshot")
	expectConnectedUserIDs(t, snapshot, guest.ID)

	// The TV leaves with its token too.
	req := leaveRequest(room.ID)
	req.Header.Set("Authorization", "Bearer "+token)
	serveOpenAPIExchange(t, app.Router, "leaveWatchRoom", req, http.StatusOK)
	readUntilEventType(t, conn, "room_access_revoked")
}

func TestWatchRoomWebSocket_OversizedFrameClosesConnection(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)

	conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer conn.Close()
	readUntilEventType(t, conn, "room_snapshot")

	padding := strings.Repeat(" ", watchRoomReadLimit)
	err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"`+padding+`}`))
	if err != nil {
		t.Fatal(err)
	}
	expectSocketToClose(t, conn)
}

func TestWatchRoomWebSocket_ClampsPositionToMovieDuration(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	_, err := app.DB.Exec(`UPDATE movies SET duration = 120 WHERE id = ?`, movieID)
	if err != nil {
		t.Fatal(err)
	}
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	server := setupWatchRoomWSTestServer(t, app)

	conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
	defer conn.Close()
	readUntilEventType(t, conn, "room_snapshot")

	err = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"seek","position_sec":1e12}`))
	if err != nil {
		t.Fatal(err)
	}
	event := readUntilEventType(t, conn, watchRoomPlaybackChangedEvent)
	if event.Playback == nil || event.Playback.PositionSec != 120 {
		t.Fatalf("playback = %+v, want position clamped to 120", event.Playback)
	}
}

// Members seeking at once must leave every socket on the state the hub
// holds: each client's last playback_changed is the hub's final position.
func TestWatchRoomWebSocket_ConcurrentCommandsArriveInHubOrder(t *testing.T) {
	app := setupTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "ordering-guest@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID, guest.ID)
	server := setupWatchRoomWSTestServer(t, app)

	conns := make([]*websocket.Conn, 0, 4)
	for _, userID := range []int64{ownerID, ownerID, guest.ID, guest.ID} {
		conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, userID)
		defer conn.Close()
		readUntilEventType(t, conn, "room_snapshot")
		conns = append(conns, conn)
	}

	const perSender = 40
	var wg sync.WaitGroup
	for i, conn := range conns[:2] {
		wg.Add(1)
		go func(sender int, conn *websocket.Conn) {
			defer wg.Done()
			for n := range perSender {
				body := fmt.Sprintf(`{"type":"seek","position_sec":%d}`, sender*1000+n)
				_ = conn.WriteMessage(websocket.TextMessage, []byte(body))
			}
		}(i, conn)
	}
	// The other two sockets send too, so commands interleave from both users.
	for i, conn := range conns[2:] {
		wg.Add(1)
		go func(sender int, conn *websocket.Conn) {
			defer wg.Done()
			for n := range perSender {
				body := fmt.Sprintf(`{"type":"seek","position_sec":%d}`, (sender+2)*1000+n)
				_ = conn.WriteMessage(websocket.TextMessage, []byte(body))
			}
		}(i, conn)
	}
	wg.Wait()

	// Every socket receives every change; read each one's last.
	for i, conn := range conns {
		var last float64
		seen := 0
		for seen < 4*perSender {
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			var event watchRoomWSTestEvent
			err := conn.ReadJSON(&event)
			if err != nil {
				t.Fatalf("socket %d after %d changes: %v", i, seen, err)
			}
			if event.Type == watchRoomPlaybackChangedEvent && event.Playback != nil {
				last = event.Playback.PositionSec
				seen++
			}
		}
		app.WatchRoomHub.mu.Lock()
		want := app.WatchRoomHub.sessions[room.ID].state.PositionSec
		app.WatchRoomHub.mu.Unlock()
		if last != want {
			t.Fatalf("socket %d ended on %v, hub holds %v", i, last, want)
		}
	}
}

func TestWatchRoomHub_ConnectRefusedAfterShutdown(t *testing.T) {
	hub := NewWatchRoomHub()
	hub.Shutdown()

	client := newWatchRoomClient(nil, 1, userSummary{ID: 1})
	if hub.connect(1, client) {
		t.Fatal("connect succeeded after shutdown")
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.sessions) != 0 {
		t.Fatalf("shutdown hub registered a session: %d", len(hub.sessions))
	}
}

// A socket that joins the hub after its room was deleted, or its member
// removed, missed the sweep that ended the others; the re-check after
// connect sends it the event the sweep would have and closes it.
func TestWatchRoomSocketStillAllowed_CatchesChangesDuringUpgrade(t *testing.T) {
	cases := []struct {
		name   string
		change func(t *testing.T, app *Application, roomID, guestID int64)
		want   string
	}{
		{
			name: "room deleted",
			change: func(t *testing.T, app *Application, roomID, _ int64) {
				err := app.deleteWatchRoom(context.Background(), roomID)
				if err != nil {
					t.Fatal(err)
				}
			},
			want: "room_deleted",
		},
		{
			name: "member removed",
			change: func(t *testing.T, app *Application, roomID, guestID int64) {
				_, err := app.Queries.RemoveWatchRoomMember(context.Background(), database.RemoveWatchRoomMemberParams{
					RoomID: roomID,
					UserID: guestID,
				})
				if err != nil {
					t.Fatal(err)
				}
			},
			want: "room_access_revoked",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := setupSessionTestApp(t)
			ownerID, movieID := createTestUserAndMovie(t, app)
			guest := createTestUser(t, app, "Guest", "upgrade-race@example.com", false)
			room := createTestRoom(t, app, ownerID, movieID)
			addMembersToRoom(t, app, room.ID, ownerID, guest.ID)

			client := newWatchRoomClient(nil, room.ID, userSummary{ID: guest.ID})
			if !app.WatchRoomHub.connect(room.ID, client) {
				t.Fatal("connect refused")
			}
			<-client.send // the snapshot

			tc.change(t, app, room.ID, guest.ID)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if app.watchRoomSocketStillAllowed(req, client) {
				t.Fatal("the socket was still allowed")
			}
			var event watchRoomWSTestEvent
			err := json.Unmarshal(<-client.send, &event)
			if err != nil {
				t.Fatal(err)
			}
			if event.Type != tc.want {
				t.Fatalf("event = %q, want %q", event.Type, tc.want)
			}
			select {
			case <-client.done:
			default:
				t.Fatal("the socket was not closed")
			}
		})
	}
}

// A logout destroys the session without changing its version, so the
// post-connect re-check must look for the session itself: a socket that
// registered after the logout's sweep would otherwise keep its control.
func TestWatchRoomCredentialCurrent_RejectsALoggedOutCookieSession(t *testing.T) {
	app := setupSessionTestApp(t)
	ownerID, _ := createTestUserAndMovie(t, app)
	cookie := newAuthSessionCookie(t, app, ownerID)
	version, err := app.Queries.GetUserSessionVersion(context.Background(), ownerID)
	if err != nil {
		t.Fatal(err)
	}
	auth := watchRoomClientAuth{sessionToken: cookie.Value, sessionVersion: version}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if !app.watchRoomCredentialCurrent(req, ownerID, auth) {
		t.Fatal("a live session was refused")
	}

	err = app.SessionManager.Store.Delete(cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if app.watchRoomCredentialCurrent(req, ownerID, auth) {
		t.Fatal("a logged-out session was still allowed")
	}
}

// An edit that loses the race with the room's deletion answers the documented
// 404, not the foreign-key failure of adding a member to a missing room.
func TestReplaceWatchRoomInvitees_RoomDeletedAnswersDeleted(t *testing.T) {
	app := setupSessionTestApp(t)
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest := createTestUser(t, app, "Guest", "race-guest@example.com", false)
	room := createTestRoom(t, app, ownerID, movieID)
	addMembersToRoom(t, app, room.ID, ownerID)
	err := app.deleteWatchRoom(context.Background(), room.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = app.replaceWatchRoomInvitees(context.Background(), room.ID, ownerID, []int64{guest.ID})
	if !errors.Is(err, errWatchRoomDeleted) {
		t.Fatalf("err = %v, want errWatchRoomDeleted", err)
	}
}

// A revoked socket's last word is room_access_revoked: the member list that
// follows goes only to the sockets still open.
func TestWatchRoomHub_RevokedClientsMissLaterBroadcasts(t *testing.T) {
	hub := NewWatchRoomHub()
	kept := newWatchRoomClient(nil, 1, userSummary{ID: 1})
	revoked := newWatchRoomClient(nil, 1, userSummary{ID: 2})
	for _, client := range []*watchRoomClient{kept, revoked} {
		if !hub.connect(1, client) {
			t.Fatal("connect refused")
		}
	}
	drain := func(client *watchRoomClient) []string {
		var types []string
		for {
			select {
			case data := <-client.send:
				var event watchRoomWSTestEvent
				if err := json.Unmarshal(data, &event); err != nil {
					t.Fatal(err)
				}
				types = append(types, event.Type)
			default:
				return types
			}
		}
	}
	drain(kept)
	drain(revoked)

	hub.revokeMembers(1, []int64{2})
	hub.membersChanged(1, []userSummary{{ID: 1}})

	if got := drain(revoked); !slices.Equal(got, []string{"room_access_revoked"}) {
		t.Fatalf("revoked client got %v", got)
	}
	if got := drain(kept); !slices.Equal(got, []string{"members_changed"}) {
		t.Fatalf("kept client got %v", got)
	}
}
