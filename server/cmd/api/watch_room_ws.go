package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
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
	Member           *watchRoomMemberSummary `json:"member,omitempty"`
	ConnectedUserIDs []int64                 `json:"connected_user_ids,omitempty"`
}

type watchRoomClient struct {
	conn      *websocket.Conn
	roomID    int64
	user      watchRoomMemberSummary
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func newWatchRoomClient(conn *websocket.Conn, roomID int64, user watchRoomMemberSummary) *watchRoomClient {
	return &watchRoomClient{
		conn:   conn,
		roomID: roomID,
		user:   user,
		send:   make(chan []byte, watchRoomSendBufferSize),
		done:   make(chan struct{}),
	}
}

type watchRoomSession struct {
	roomID       int64
	clients      map[*watchRoomClient]bool
	connectedIDs map[int64]bool
	state        watchRoomPlaybackState
}

type WatchRoomHub struct {
	mu       sync.Mutex
	sessions map[int64]*watchRoomSession
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
	if position < watchRoomPositionDriftFloor {
		return watchRoomPositionDriftFloor
	}
	return position
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
		roomID:       roomID,
		clients:      make(map[*watchRoomClient]bool),
		connectedIDs: make(map[int64]bool),
		state: watchRoomPlaybackState{
			Paused:      true,
			PositionSec: 0,
			UpdatedAt:   now,
		},
	}
	hub.sessions[roomID] = session
	return session
}

func (hub *WatchRoomHub) connect(roomID int64, client *watchRoomClient) (watchRoomServerEvent, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session := hub.getOrCreateSession(roomID)
	_, alreadyConnected := session.connectedIDs[client.user.ID]
	session.clients[client] = true
	session.connectedIDs[client.user.ID] = true

	return hub.buildSnapshotLocked(roomID, session, time.Now().UTC()), !alreadyConnected
}

func (hub *WatchRoomHub) disconnect(client *watchRoomClient) *watchRoomServerEvent {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[client.roomID]
	if !ok {
		return nil
	}

	delete(session.clients, client)
	stillConnected := false
	for existing := range session.clients {
		if existing.user.ID == client.user.ID {
			stillConnected = true
			break
		}
	}
	if !stillConnected {
		delete(session.connectedIDs, client.user.ID)
	} else {
		return nil
	}

	if len(session.clients) == 0 {
		delete(hub.sessions, client.roomID)
		return nil
	}

	return &watchRoomServerEvent{
		Type:             "member_left",
		RoomID:           client.roomID,
		Member:           &client.user,
		ConnectedUserIDs: connectedUserIDs(session.connectedIDs),
	}
}

func (hub *WatchRoomHub) memberJoinedEvent(roomID int64, member watchRoomMemberSummary) *watchRoomServerEvent {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[roomID]
	if !ok {
		return nil
	}

	return &watchRoomServerEvent{
		Type:             "member_joined",
		RoomID:           roomID,
		Member:           &member,
		ConnectedUserIDs: connectedUserIDs(session.connectedIDs),
	}
}

func (hub *WatchRoomHub) applyPlaybackEvent(roomID int64, eventType string, positionSec float64) (*watchRoomServerEvent, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[roomID]
	if !ok {
		return nil, false
	}

	now := time.Now().UTC()
	currentPosition := session.state.currentPosition(now)

	switch eventType {
	case "play":
		if positionSec < 0 {
			positionSec = currentPosition
		}
		session.state = watchRoomPlaybackState{
			Paused:      false,
			PositionSec: positionSec,
			UpdatedAt:   now,
		}
	case "pause":
		if positionSec < 0 {
			positionSec = currentPosition
		}
		session.state = watchRoomPlaybackState{
			Paused:      true,
			PositionSec: positionSec,
			UpdatedAt:   now,
		}
	case "seek":
		if positionSec < 0 {
			positionSec = currentPosition
		}
		session.state = watchRoomPlaybackState{
			Paused:      session.state.Paused,
			PositionSec: positionSec,
			UpdatedAt:   now,
		}
	default:
		return nil, false
	}

	snapshot := session.state.snapshot(now)
	return &watchRoomServerEvent{
		Type:     "playback_changed",
		RoomID:   roomID,
		Playback: &snapshot,
	}, true
}

func (hub *WatchRoomHub) broadcast(roomID int64, payload watchRoomServerEvent) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	hub.mu.Lock()
	session, ok := hub.sessions[roomID]
	if !ok {
		hub.mu.Unlock()
		return
	}

	clients := make([]*watchRoomClient, 0, len(session.clients))
	for client := range session.clients {
		clients = append(clients, client)
	}
	hub.mu.Unlock()

	for _, client := range clients {
		client.enqueue(data)
	}
}

func (hub *WatchRoomHub) broadcastToOthers(roomID int64, sender *watchRoomClient, payload watchRoomServerEvent) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	hub.mu.Lock()
	session, ok := hub.sessions[roomID]
	if !ok {
		hub.mu.Unlock()
		return
	}

	clients := make([]*watchRoomClient, 0, len(session.clients))
	for client := range session.clients {
		if client != sender {
			clients = append(clients, client)
		}
	}
	hub.mu.Unlock()

	for _, client := range clients {
		client.enqueue(data)
	}
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

func pointerToPlaybackState(state watchRoomPlaybackState) *watchRoomPlaybackState {
	copy := state
	return &copy
}

func (hub *WatchRoomHub) buildSnapshotLocked(roomID int64, session *watchRoomSession, now time.Time) watchRoomServerEvent {
	return watchRoomServerEvent{
		Type:             "room_snapshot",
		RoomID:           roomID,
		Playback:         pointerToPlaybackState(session.state.snapshot(now)),
		ConnectedUserIDs: connectedUserIDs(session.connectedIDs),
	}
}

func (hub *WatchRoomHub) currentSnapshot(roomID int64) *watchRoomServerEvent {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	session, ok := hub.sessions[roomID]
	if !ok {
		return nil
	}

	snapshot := hub.buildSnapshotLocked(roomID, session, time.Now().UTC())
	return &snapshot
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
	viteURL := strings.TrimSpace(os.Getenv("VITE_DEV_SERVER"))
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

	if originURL.Host != r.Host {
		return isAllowedWatchRoomDevOrigin(originURL) ||
			isAllowedWatchRoomLocalDevOrigin(originURL, r.Host)
	}

	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}

	if sameWatchRoomOrigin(originURL, expectedScheme, r.Host) {
		return true
	}

	return isAllowedWatchRoomDevOrigin(originURL) ||
		isAllowedWatchRoomLocalDevOrigin(originURL, r.Host)
}

var watchRoomUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return isAllowedWatchRoomOrigin(r)
	},
}

func (app *Application) loadAuthorizedWatchRoom(ctx context.Context, roomID, userID int64) (database.WatchRoom, error) {
	room, err := app.Queries.GetWatchRoomByID(ctx, roomID)
	if err != nil {
		return database.WatchRoom{}, err
	}

	isMember, err := app.Queries.IsWatchRoomMember(ctx, database.IsWatchRoomMemberParams{
		RoomID: roomID,
		UserID: userID,
	})
	if err != nil {
		return database.WatchRoom{}, err
	}
	if !isMember {
		// Callers treat sql.ErrNoRows as "no access", matching the room lookup.
		return database.WatchRoom{}, sql.ErrNoRows
	}

	return room, nil
}

func (app *Application) loadAuthorizedWatchRoomForRequest(w http.ResponseWriter, r *http.Request) (database.WatchRoom, int64, bool) {
	userID, ok := app.currentUserID(w, r)
	if !ok {
		return database.WatchRoom{}, 0, false
	}

	roomID, err := parseRoomID(r)
	if err != nil {
		helpers.ErrorJSON(w, err, http.StatusBadRequest)
		return database.WatchRoom{}, 0, false
	}

	room, err := app.loadAuthorizedWatchRoom(r.Context(), roomID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.ErrorJSON(w, errors.New("access denied"), http.StatusForbidden)
			return database.WatchRoom{}, 0, false
		}
		app.Logger.Error("failed to authorize watch room", "error", err, "room_id", roomID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return database.WatchRoom{}, 0, false
	}

	return room, userID, true
}

func (app *Application) loadRoomMemberSummary(ctx context.Context, roomID, userID int64) (watchRoomMemberSummary, error) {
	row, err := app.Queries.GetWatchRoomMemberByUserID(ctx, database.GetWatchRoomMemberByUserIDParams{
		RoomID: roomID,
		UserID: userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return watchRoomMemberSummary{}, errors.New("member not found")
		}
		return watchRoomMemberSummary{}, err
	}

	var avatar *string
	if row.Avatar.Valid {
		avatar = &row.Avatar.String
	}

	return watchRoomMemberSummary{
		ID:     row.ID,
		Name:   row.Name,
		Avatar: avatar,
	}, nil
}

// Only room members may upgrade.
func (app *Application) WatchRoomWebSocket(w http.ResponseWriter, r *http.Request) {
	room, userID, ok := app.loadAuthorizedWatchRoomForRequest(w, r)
	if !ok {
		return
	}

	member, err := app.loadRoomMemberSummary(r.Context(), room.ID, userID)
	if err != nil {
		app.Logger.Error("failed to load room member summary for websocket", "error", err, "room_id", room.ID, "user_id", userID)
		helpers.ErrorJSON(w, errors.New(internalServerErrorMessage))
		return
	}

	conn, err := watchRoomUpgrader.Upgrade(w, r, nil)
	if err != nil {
		app.Logger.Error("failed to upgrade watch room websocket", "error", err, "room_id", room.ID, "user_id", userID)
		return
	}

	if app.Wait != nil {
		app.Wait.Add(1)
	}

	client := newWatchRoomClient(conn, room.ID, member)
	go client.writePump()

	snapshot, firstConnection := app.WatchRoomHub.connect(room.ID, client)
	client.enqueueEvent(snapshot)
	if firstConnection {
		if event := app.WatchRoomHub.memberJoinedEvent(room.ID, member); event != nil {
			app.WatchRoomHub.broadcastToOthers(room.ID, client, *event)
		}
	}

	defer func() {
		if app.Wait != nil {
			app.Wait.Done()
		}
		if event := app.WatchRoomHub.disconnect(client); event != nil {
			app.WatchRoomHub.broadcast(room.ID, *event)
		}
		client.close()
	}()

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
			if fresh := app.WatchRoomHub.currentSnapshot(room.ID); fresh != nil {
				client.enqueueEvent(*fresh)
			}
		case "ping":
			client.enqueueEvent(watchRoomServerEvent{Type: "pong", RoomID: room.ID})
		case "play", "pause", "seek":
			positionSec := -1.0
			if event.PositionSec != nil {
				positionSec = *event.PositionSec
			}
			update, ok := app.WatchRoomHub.applyPlaybackEvent(room.ID, event.Type, positionSec)
			if ok && update != nil {
				app.WatchRoomHub.broadcast(room.ID, *update)
			}
		}
	}
}
