package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/gorilla/websocket"
)

const watchRoomPositionDriftFloor = 0.0

const (
	watchRoomWriteTimeout = 5 * time.Second
	// Deep enough to absorb legitimate broadcast bursts (e.g. several
	// members seeking rapidly at once) without evicting a healthy client
	// whose writer is momentarily behind; a truly stalled consumer stops
	// draining entirely and still overflows it.
	watchRoomSendBufferSize = 256
)

// Vars rather than consts so tests can shrink them.
var (
	watchRoomReadTimeout = 60 * time.Second
	// Must be shorter than watchRoomReadTimeout so pongs keep idle
	// connections alive even when the client's JS timers are throttled.
	watchRoomPingInterval = 40 * time.Second
)

type watchRoomPlaybackState struct {
	Paused      bool      `json:"paused"`
	PositionSec float64   `json:"position_sec"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type watchRoomClientEvent struct {
	Type        string   `json:"type"`
	PositionSec *float64 `json:"position_sec,omitempty"`
}

type watchRoomServerEvent struct {
	Type             string                  `json:"type"`
	RoomID           int64                   `json:"room_id"`
	Playback         *watchRoomPlaybackState `json:"playback,omitempty"`
	Member           *userSummary            `json:"member,omitempty"`
	Members          []userSummary           `json:"members,omitempty"`
	ConnectedUserIDs []int64                 `json:"connected_user_ids,omitempty"`
}

// watchRoomClientAuth records how a socket authenticated, so revoking that
// credential (logout, device revocation, a password change) can close it. A
// socket outlives the request that opened it, and nothing else re-checks it.
type watchRoomClientAuth struct {
	// deviceID is set for a device bearer token, 0 for a cookie session.
	deviceID       int64
	sessionToken   string
	sessionVersion int64
}

type watchRoomClient struct {
	conn      *websocket.Conn
	roomID    int64
	user      userSummary
	auth      watchRoomClientAuth
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func newWatchRoomClient(conn *websocket.Conn, roomID int64, user userSummary) *watchRoomClient {
	return &watchRoomClient{
		conn:   conn,
		roomID: roomID,
		user:   user,
		send:   make(chan []byte, watchRoomSendBufferSize),
		done:   make(chan struct{}),
	}
}

type watchRoomSession struct {
	clients      map[*watchRoomClient]bool
	connectedIDs map[int64]bool
	state        watchRoomPlaybackState
}

// WatchRoomHub holds every open room socket. Each change to a room's state
// queues its events while still holding mu: enqueue never blocks, and queueing
// under the lock is what keeps every client's events in the order the hub
// applied them.
type WatchRoomHub struct {
	mu       sync.Mutex
	sessions map[int64]*watchRoomSession
	// closed is set by Shutdown; a socket that finishes its upgrade after it
	// is refused, so the shutdown wait cannot hang on it.
	closed bool
}

func NewWatchRoomHub() *WatchRoomHub {
	return &WatchRoomHub{
		sessions: make(map[int64]*watchRoomSession),
	}
}

func (state watchRoomPlaybackState) currentPosition(now time.Time) float64 {
	position := state.PositionSec
	if !state.Paused {
		position += now.Sub(state.UpdatedAt).Seconds()
	}
	return max(position, watchRoomPositionDriftFloor)
}

func (state watchRoomPlaybackState) snapshot(now time.Time) watchRoomPlaybackState {
	return watchRoomPlaybackState{
		Paused:      state.Paused,
		PositionSec: state.currentPosition(now),
		UpdatedAt:   now.UTC(),
	}
}

func (hub *WatchRoomHub) getOrCreateSession(roomID int64) *watchRoomSession {
	session, ok := hub.sessions[roomID]
	if ok {
		return session
	}

	now := time.Now().UTC()
	session = &watchRoomSession{
		clients:      make(map[*watchRoomClient]bool),
		connectedIDs: make(map[int64]bool),
		state: watchRoomPlaybackState{
			Paused:    true,
			UpdatedAt: now,
		},
	}
	hub.sessions[roomID] = session
	return session
}

// broadcastLocked queues payload for every open client in the session but
// sender; a nil sender reaches everyone. A client already closed (revoked,
// evicted) is skipped: its read loop removes it shortly, and what it was last
// told stays its last word. The caller holds hub.mu.
func broadcastLocked(session *watchRoomSession, sender *watchRoomClient, payload watchRoomServerEvent) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	for client := range session.clients {
		if client == sender || client.closed() {
			continue
		}
		client.enqueue(data)
	}
}

// connect registers the client, queues its snapshot, and tells the others
// when this is the user's first connection, all under one lock hold, so no
// playback change can reach the client ahead of its snapshot. It reports
// false once the hub has shut down.
func (hub *WatchRoomHub) connect(roomID int64, client *watchRoomClient) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return false
	}

	session := hub.getOrCreateSession(roomID)
	_, alreadyConnected := session.connectedIDs[client.user.ID]
	session.clients[client] = true
	session.connectedIDs[client.user.ID] = true

	client.enqueueEvent(hub.buildSnapshotLocked(roomID, session, time.Now().UTC()))
	if !alreadyConnected {
		member := client.user
		broadcastLocked(session, client, watchRoomServerEvent{
			Type:             "member_joined",
			RoomID:           roomID,
			Member:           &member,
			ConnectedUserIDs: connectedUserIDs(session.connectedIDs),
		})
	}
	return true
}

// disconnect removes the client and, when it was the user's last connection
// while others remain, tells them the user left.
func (hub *WatchRoomHub) disconnect(client *watchRoomClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[client.roomID]
	if !ok {
		return
	}

	_, registered := session.clients[client]
	if !registered {
		return
	}
	delete(session.clients, client)
	for existing := range session.clients {
		if existing.user.ID == client.user.ID {
			return
		}
	}
	delete(session.connectedIDs, client.user.ID)

	if len(session.clients) == 0 {
		delete(hub.sessions, client.roomID)
		return
	}

	member := client.user
	broadcastLocked(session, nil, watchRoomServerEvent{
		Type:             "member_left",
		RoomID:           client.roomID,
		Member:           &member,
		ConnectedUserIDs: connectedUserIDs(session.connectedIDs),
	})
}

// applyPlaybackEvent updates the sender's room's playback state and queues the
// change for every client, the sender included. maxPositionSec is the movie's
// duration, or 0 when it is unknown; a position past it is clamped to it. A
// sender already closed (revoked, evicted) or no longer in the room changes
// nothing: its read loop can still hand over a command read after revokeMembers
// closed it, and the check shares the lock that revocation takes.
func (hub *WatchRoomHub) applyPlaybackEvent(sender *watchRoomClient, eventType string, positionSec, maxPositionSec float64) (*watchRoomServerEvent, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	roomID := sender.roomID
	session, ok := hub.sessions[roomID]
	if !ok {
		return nil, false
	}
	if _, registered := session.clients[sender]; !registered || sender.closed() {
		return nil, false
	}

	now := time.Now().UTC()
	paused := session.state.Paused
	switch eventType {
	case "play":
		paused = false
	case "pause":
		paused = true
	case "seek":
		// A seek keeps the current paused state.
	default:
		return nil, false
	}

	if positionSec < 0 {
		positionSec = session.state.currentPosition(now)
	}
	if maxPositionSec > 0 && positionSec > maxPositionSec {
		positionSec = maxPositionSec
	}
	session.state = watchRoomPlaybackState{Paused: paused, PositionSec: positionSec, UpdatedAt: now}

	snapshot := session.state.snapshot(now)
	event := &watchRoomServerEvent{
		Type:     "playback_changed",
		RoomID:   roomID,
		Playback: &snapshot,
	}
	broadcastLocked(session, nil, *event)
	return event, true
}

// membersChanged tells every open socket in the room its new member list.
func (hub *WatchRoomHub) membersChanged(roomID int64, members []userSummary) {
	if hub == nil {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[roomID]
	if !ok {
		return
	}
	broadcastLocked(session, nil, watchRoomServerEvent{
		Type:    "members_changed",
		RoomID:  roomID,
		Members: members,
	})
}

// revokeMembers sends room_access_revoked to every socket the given users have
// open in the room, then closes them. Each socket's read loop then removes it,
// which tells the rest of the room the user left.
func (hub *WatchRoomHub) revokeMembers(roomID int64, userIDs []int64) {
	if hub == nil || len(userIDs) == 0 {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[roomID]
	if !ok {
		return
	}
	data, err := json.Marshal(watchRoomServerEvent{Type: "room_access_revoked", RoomID: roomID})
	if err != nil {
		return
	}
	for client := range session.clients {
		if slices.Contains(userIDs, client.user.ID) {
			client.enqueue(data)
			client.close()
		}
	}
}

// disconnectMatching closes every socket match selects, in any room, without
// a message: the credential it opened with is gone, so its reconnect is
// refused with a 401 and the client signs out.
func (hub *WatchRoomHub) disconnectMatching(match func(*watchRoomClient) bool) {
	if hub == nil {
		return
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for _, session := range hub.sessions {
		for client := range session.clients {
			if match(client) {
				client.close()
			}
		}
	}
}

// disconnectDevice closes the sockets a revoked device token opened.
func (hub *WatchRoomHub) disconnectDevice(deviceID int64) {
	hub.disconnectMatching(func(client *watchRoomClient) bool {
		return client.auth.deviceID == deviceID
	})
}

// disconnectUserDevices closes every socket a user opened with a device token.
func (hub *WatchRoomHub) disconnectUserDevices(userID int64) {
	hub.disconnectMatching(func(client *watchRoomClient) bool {
		return client.auth.deviceID != 0 && client.user.ID == userID
	})
}

// disconnectStaleCookieSessions closes the user's cookie sockets whose session
// recorded a version other than current, the same rule IsAuth applies.
func (hub *WatchRoomHub) disconnectStaleCookieSessions(userID, current int64) {
	hub.disconnectMatching(func(client *watchRoomClient) bool {
		return client.auth.deviceID == 0 && client.user.ID == userID &&
			client.auth.sessionVersion != current
	})
}

// disconnectSessionToken closes the sockets a logged-out cookie session opened.
func (hub *WatchRoomHub) disconnectSessionToken(token string) {
	if token == "" {
		return
	}
	hub.disconnectMatching(func(client *watchRoomClient) bool {
		return client.auth.deviceID == 0 && client.auth.sessionToken == token
	})
}

func (hub *WatchRoomHub) snapshotAndClearClients(roomID int64) []*watchRoomClient {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[roomID]
	if !ok {
		return nil
	}

	clients := make([]*watchRoomClient, 0, len(session.clients))
	for client := range session.clients {
		clients = append(clients, client)
	}
	delete(hub.sessions, roomID)
	return clients
}

func (hub *WatchRoomHub) deleteRoom(roomID int64) {
	clients := hub.snapshotAndClearClients(roomID)
	if len(clients) == 0 {
		return
	}

	data, err := json.Marshal(watchRoomServerEvent{
		Type:   "room_deleted",
		RoomID: roomID,
	})

	for _, client := range clients {
		if err == nil {
			client.enqueue(data)
		}
		client.close()
	}
}

func (hub *WatchRoomHub) Shutdown() {
	hub.mu.Lock()
	hub.closed = true
	clients := make([]*watchRoomClient, 0)
	for _, session := range hub.sessions {
		for client := range session.clients {
			clients = append(clients, client)
		}
	}
	hub.sessions = make(map[int64]*watchRoomSession)
	hub.mu.Unlock()

	for _, client := range clients {
		client.close()
	}
}

// close signals the writer goroutine to flush queued payloads and shut the
// connection down. Safe to call from any goroutine, any number of times.
func (client *watchRoomClient) close() {
	client.closeOnce.Do(func() {
		close(client.done)
	})
}

// closed reports whether close has been called.
func (client *watchRoomClient) closed() bool {
	select {
	case <-client.done:
		return true
	default:
		return false
	}
}

// enqueue queues a payload without blocking. A full outbox means the peer has
// stalled; it gets evicted instead of delaying the rest of the room.
func (client *watchRoomClient) enqueue(payload []byte) {
	select {
	case client.send <- payload:
	default:
		client.close()
	}
}

func (client *watchRoomClient) enqueueEvent(payload watchRoomServerEvent) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	client.enqueue(data)
}

func (client *watchRoomClient) write(messageType int, payload []byte) bool {
	_ = client.conn.SetWriteDeadline(time.Now().Add(watchRoomWriteTimeout))
	return client.conn.WriteMessage(messageType, payload) == nil
}

// writePump is the sole writer for the connection. It drains the outbox,
// keeps the peer alive with ping control frames, and on shutdown flushes any
// queued payloads (e.g. room_deleted) before closing the connection, which
// in turn unblocks the read loop.
func (client *watchRoomClient) writePump() {
	ticker := time.NewTicker(watchRoomPingInterval)
	defer func() {
		ticker.Stop()
		client.close()
		_ = client.conn.Close()
	}()

	for {
		select {
		case payload := <-client.send:
			if !client.write(websocket.TextMessage, payload) {
				return
			}
		case <-ticker.C:
			if !client.write(websocket.PingMessage, nil) {
				return
			}
		case <-client.done:
			for {
				select {
				case payload := <-client.send:
					if !client.write(websocket.TextMessage, payload) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

func (hub *WatchRoomHub) buildSnapshotLocked(roomID int64, session *watchRoomSession, now time.Time) watchRoomServerEvent {
	playback := session.state.snapshot(now)
	return watchRoomServerEvent{
		Type:             "room_snapshot",
		RoomID:           roomID,
		Playback:         &playback,
		ConnectedUserIDs: connectedUserIDs(session.connectedIDs),
	}
}

// sendSnapshot queues the client's room's current snapshot, built and queued
// under one lock hold, so no playback change applied meanwhile can reach the
// client ahead of the older snapshot.
func (hub *WatchRoomHub) sendSnapshot(client *watchRoomClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[client.roomID]
	if !ok {
		return
	}
	client.enqueueEvent(hub.buildSnapshotLocked(client.roomID, session, time.Now().UTC()))
}

func connectedUserIDs(ids map[int64]bool) []int64 {
	userIDs := make([]int64, 0, len(ids))
	for id := range ids {
		userIDs = append(userIDs, id)
	}
	slices.Sort(userIDs)
	return userIDs
}

func sameWatchRoomOrigin(originURL *url.URL, expectedScheme string, expectedHost string) bool {
	return strings.EqualFold(originURL.Scheme, expectedScheme) &&
		strings.EqualFold(originURL.Host, expectedHost)
}

func isLocalWatchRoomDevHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func isAllowedWatchRoomDevOrigin(originURL *url.URL) bool {
	viteURL := viteDevServerURL()
	if viteURL == "" {
		return false
	}

	viteOrigin, err := url.Parse(viteURL)
	if err != nil {
		return false
	}

	if viteOrigin.Scheme == "" || viteOrigin.Host == "" {
		return false
	}

	return sameWatchRoomOrigin(originURL, viteOrigin.Scheme, viteOrigin.Host)
}

func isAllowedWatchRoomLocalDevOrigin(originURL *url.URL, requestHost string) bool {
	if originURL.Scheme != "http" {
		return false
	}

	if originURL.Port() != "3000" {
		return false
	}

	requestHostName := requestHost
	parsedHost, _, err := net.SplitHostPort(requestHost)
	if err == nil {
		requestHostName = parsedHost
	}

	return isLocalWatchRoomDevHost(originURL.Hostname()) &&
		isLocalWatchRoomDevHost(requestHostName) &&
		strings.EqualFold(originURL.Hostname(), requestHostName)
}

func isAllowedWatchRoomOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// The page's own host, over either web scheme. Behind a TLS-terminating
	// proxy that keeps the Host header the page is https while this hop is
	// http, so the scheme cannot be matched against r.TLS; a cross-site page
	// cannot put this host in its Origin either way.
	if sameWatchRoomOrigin(originURL, "https", r.Host) ||
		sameWatchRoomOrigin(originURL, "http", r.Host) {
		return true
	}

	return isAllowedWatchRoomDevOrigin(originURL) ||
		isAllowedWatchRoomLocalDevOrigin(originURL, r.Host)
}

var watchRoomUpgrader = websocket.Upgrader{
	CheckOrigin: isAllowedWatchRoomOrigin,
}

// watchRoomReadLimit caps one incoming frame. The largest valid client event
// is a few dozen bytes; anything past this closes the socket.
const watchRoomReadLimit = 4 << 10

// Only room members may upgrade. One joined query authorizes the member and
// returns the presence summary the socket broadcasts; sql.ErrNoRows means "no
// room or no access", same as the HTTP media paths.
func (app *Application) WatchRoomWebSocket(w http.ResponseWriter, r *http.Request) {
	userID := app.userIDFromRequest(r)

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return
	}

	row, err := app.Queries.GetWatchRoomForMemberWithSummary(r.Context(), database.GetWatchRoomForMemberWithSummaryParams{
		ID:     roomID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New(accessDeniedMessage), http.StatusForbidden)
			return
		}
		app.serverError(w, err, "failed to authorize watch room websocket", internalServerErrorMessage,
			"room_id", roomID, "user_id", userID)
		return
	}

	member := userSummary{
		ID:     userID,
		Name:   row.MemberName,
		Avatar: helpers.StringPtrFromNull(row.MemberAvatar),
	}
	// 0 leaves positions unclamped when the scan recorded no duration.
	maxPositionSec := 0.0
	if row.MovieDuration.Valid && row.MovieDuration.Float64 > 0 {
		maxPositionSec = row.MovieDuration.Float64
	}

	conn, err := watchRoomUpgrader.Upgrade(w, r, nil)
	if err != nil {
		app.Logger.Error("failed to upgrade watch room websocket", "error", err, "room_id", row.ID, "user_id", userID)
		return
	}
	conn.SetReadLimit(watchRoomReadLimit)

	app.Wait.Add(1)

	client := newWatchRoomClient(conn, row.ID, member)
	client.auth = app.watchRoomClientAuth(r)
	go client.writePump()

	defer func() {
		app.Wait.Done()
		app.WatchRoomHub.disconnect(client)
		client.close()
	}()

	if !app.WatchRoomHub.connect(row.ID, client) {
		return
	}

	// The room, the membership or the credential may have gone between the
	// check above and connect; the sweep that ended them ran before this
	// socket was in the hub, so check again now that it is.
	if !app.watchRoomSocketStillAllowed(r, client) {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(watchRoomReadTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(watchRoomReadTimeout))
	})

	for {
		_, rawMessage, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(watchRoomReadTimeout))

		var event watchRoomClientEvent
		err = json.Unmarshal(rawMessage, &event)
		if err != nil {
			continue
		}

		switch event.Type {
		case "join":
			app.WatchRoomHub.sendSnapshot(client)
		case "ping":
			client.enqueueEvent(watchRoomServerEvent{Type: "pong", RoomID: row.ID})
		case "play", "pause", "seek":
			positionSec := -1.0
			if event.PositionSec != nil {
				positionSec = *event.PositionSec
			}
			app.WatchRoomHub.applyPlaybackEvent(client, event.Type, positionSec, maxPositionSec)
		}
	}
}

// watchRoomClientAuth records the credential the upgrade request carried.
func (app *Application) watchRoomClientAuth(r *http.Request) watchRoomClientAuth {
	device := deviceAuthFrom(r.Context())
	if device != nil {
		return watchRoomClientAuth{deviceID: device.DeviceID}
	}
	return watchRoomClientAuth{
		sessionToken:   app.SessionManager.Token(r.Context()),
		sessionVersion: app.SessionManager.GetInt64(r.Context(), cookieSessionVersion),
	}
}

// watchRoomSocketStillAllowed re-checks a just-connected socket. A room that
// is gone gets room_deleted, a member who was removed gets
// room_access_revoked, and a credential that was revoked gets no message;
// each of those closes the socket and reports false.
func (app *Application) watchRoomSocketStillAllowed(r *http.Request, client *watchRoomClient) bool {
	ctx := r.Context()

	isMember, err := app.Queries.IsWatchRoomMember(ctx, database.IsWatchRoomMemberParams{
		RoomID: client.roomID,
		UserID: client.user.ID,
	})
	if err != nil {
		app.Logger.Error("failed to re-check watch room membership", "error", err,
			"room_id", client.roomID, "user_id", client.user.ID)
		client.close()
		return false
	}
	if !isMember {
		eventType := "room_access_revoked"
		_, err = app.Queries.GetWatchRoomByID(ctx, client.roomID)
		if errors.Is(err, sql.ErrNoRows) {
			eventType = "room_deleted"
		}
		client.enqueueEvent(watchRoomServerEvent{Type: eventType, RoomID: client.roomID})
		client.close()
		return false
	}

	if !app.watchRoomCredentialCurrent(r, client.user.ID, client.auth) {
		client.close()
		return false
	}
	return true
}

// watchRoomCredentialCurrent reports whether the socket's credential still
// authenticates: the device row still exists, or the cookie session still
// exists (a logout destroys it without changing the version) and its recorded
// version is still the user's.
func (app *Application) watchRoomCredentialCurrent(r *http.Request, userID int64, auth watchRoomClientAuth) bool {
	if auth.deviceID != 0 {
		token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		device, err := app.Queries.GetDeviceByTokenHash(r.Context(), hashDeviceToken(token))
		return err == nil && device.ID == auth.deviceID
	}

	if auth.sessionToken == "" {
		return false
	}
	_, found, err := app.SessionManager.Store.Find(auth.sessionToken)
	if err != nil || !found {
		return false
	}

	current, err := app.currentSessionVersion(r.Context(), userID)
	return err == nil && current == auth.sessionVersion
}
