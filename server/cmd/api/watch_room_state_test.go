package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"igloo/cmd/internal/database"

	"github.com/gorilla/websocket"
)

func TestWatchRoomPlaybackState_CurrentPosition(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name  string
		state watchRoomPlaybackState
		at    time.Time
		min   float64
		max   float64
	}{
		{
			name: "paused state does not extrapolate",
			state: watchRoomPlaybackState{
				Paused:      true,
				PositionSec: 42,
				UpdatedAt:   now.Add(-10 * time.Second),
			},
			at:  now,
			min: 42,
			max: 42,
		},
		{
			name: "playing state adds elapsed wall clock",
			state: watchRoomPlaybackState{
				Paused:      false,
				PositionSec: 42,
				UpdatedAt:   now.Add(-10 * time.Second),
			},
			at:  now,
			min: 52,
			max: 52,
		},
		{
			name: "position is clamped at the drift floor",
			state: watchRoomPlaybackState{
				Paused:      true,
				PositionSec: -5,
				UpdatedAt:   now,
			},
			at:  now,
			min: watchRoomPositionDriftFloor,
			max: watchRoomPositionDriftFloor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.currentPosition(tt.at)
			if got < tt.min || got > tt.max {
				t.Fatalf("currentPosition() = %.3f, want in [%.3f, %.3f]", got, tt.min, tt.max)
			}
		})
	}
}

func TestWatchRoomPlaybackState_SnapshotRebasesUpdatedAt(t *testing.T) {
	now := time.Now().UTC()
	state := watchRoomPlaybackState{
		Paused:      false,
		PositionSec: 10,
		UpdatedAt:   now.Add(-4 * time.Second),
	}

	snap := state.snapshot(now)
	if snap.Paused {
		t.Fatal("snapshot flipped paused state")
	}
	if snap.PositionSec < 13.9 || snap.PositionSec > 14.1 {
		t.Fatalf("snapshot position = %.3f, want ~14", snap.PositionSec)
	}
	if !snap.UpdatedAt.Equal(now) {
		t.Fatalf("snapshot UpdatedAt = %v, want %v", snap.UpdatedAt, now)
	}
}

func TestWatchRoomHub_ApplyPlaybackEventTransitions(t *testing.T) {
	newHubWithSession := func(roomID int64) *WatchRoomHub {
		hub := NewWatchRoomHub()
		hub.mu.Lock()
		hub.getOrCreateSession(roomID)
		hub.mu.Unlock()
		return hub
	}

	t.Run("play sets position and unpauses", func(t *testing.T) {
		hub := newHubWithSession(1)
		event, ok := hub.applyPlaybackEvent(1, "play", 30)
		if !ok || event == nil || event.Playback == nil {
			t.Fatalf("applyPlaybackEvent(play) = %+v, %v", event, ok)
		}
		if event.Type != "playback_changed" || event.Playback.Paused || event.Playback.PositionSec != 30 {
			t.Fatalf("unexpected play event: %+v", event.Playback)
		}
	})

	t.Run("pause keeps position and pauses", func(t *testing.T) {
		hub := newHubWithSession(1)
		_, _ = hub.applyPlaybackEvent(1, "play", 30)
		event, ok := hub.applyPlaybackEvent(1, "pause", 31)
		if !ok || event == nil || !event.Playback.Paused || event.Playback.PositionSec != 31 {
			t.Fatalf("unexpected pause event: %+v ok=%v", event, ok)
		}
	})

	t.Run("seek repositions without changing paused state", func(t *testing.T) {
		hub := newHubWithSession(1)
		_, _ = hub.applyPlaybackEvent(1, "play", 5)
		event, ok := hub.applyPlaybackEvent(1, "seek", 90)
		if !ok || event == nil || event.Playback.Paused {
			t.Fatalf("seek while playing should stay unpaused: %+v ok=%v", event, ok)
		}
		if event.Playback.PositionSec < 90 || event.Playback.PositionSec > 90.5 {
			t.Fatalf("seek position = %.3f, want ~90", event.Playback.PositionSec)
		}

		_, _ = hub.applyPlaybackEvent(1, "pause", 90)
		event, ok = hub.applyPlaybackEvent(1, "seek", 10)
		if !ok || event == nil || !event.Playback.Paused || event.Playback.PositionSec != 10 {
			t.Fatalf("seek while paused should stay paused: %+v ok=%v", event, ok)
		}
	})

	t.Run("missing position falls back to extrapolated current position", func(t *testing.T) {
		hub := newHubWithSession(1)
		_, _ = hub.applyPlaybackEvent(1, "play", 20)
		time.Sleep(50 * time.Millisecond)
		event, ok := hub.applyPlaybackEvent(1, "pause", -1)
		if !ok || event == nil || !event.Playback.Paused {
			t.Fatalf("unexpected fallback pause event: %+v ok=%v", event, ok)
		}
		if event.Playback.PositionSec <= 20 || event.Playback.PositionSec > 21 {
			t.Fatalf("fallback position = %.3f, want just past 20", event.Playback.PositionSec)
		}
	})

	t.Run("unknown event type is rejected", func(t *testing.T) {
		hub := newHubWithSession(1)
		event, ok := hub.applyPlaybackEvent(1, "warp", 10)
		if ok || event != nil {
			t.Fatalf("expected unknown event to be rejected, got %+v ok=%v", event, ok)
		}
	})

	t.Run("unknown room is rejected", func(t *testing.T) {
		hub := NewWatchRoomHub()
		event, ok := hub.applyPlaybackEvent(404, "play", 10)
		if ok || event != nil {
			t.Fatalf("expected unknown room to be rejected, got %+v ok=%v", event, ok)
		}
	})
}

func TestWatchRoomWebSocket_RoomsAreIsolated(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerAID, movieID := createTestUserAndMovie(t, app)
	ownerB, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Owner B",
		Email:    "owner-b-isolated@example.com",
		Password: "hashed",
	})
	if err != nil {
		t.Fatalf("create owner b: %v", err)
	}

	roomA := createTestRoom(t, app, ownerAID, movieID)
	addMembersToRoom(t, app, roomA.ID, ownerAID)
	roomB := createTestRoom(t, app, ownerB.ID, movieID)
	addMembersToRoom(t, app, roomB.ID, ownerB.ID)

	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	connA, _ := dialWatchRoomSocket(t, app, server.URL, roomA.ID, ownerAID)
	defer connA.Close()
	connB, _ := dialWatchRoomSocket(t, app, server.URL, roomB.ID, ownerB.ID)
	defer connB.Close()

	_ = readUntilEventType(t, connA, "room_snapshot")
	_ = readUntilEventType(t, connB, "room_snapshot")

	if err := connA.WriteJSON(map[string]any{"type": "play", "position_sec": 25}); err != nil {
		t.Fatalf("room A play event: %v", err)
	}

	playEvent := readUntilEventType(t, connA, "playback_changed")
	if playEvent.RoomID != roomA.ID {
		t.Fatalf("playback_changed room_id = %d, want %d", playEvent.RoomID, roomA.ID)
	}

	expectNoEventType(t, connB, "playback_changed")
}

func TestWatchRoomWebSocket_ConcurrentPlaybackEventsDoNotDeadlock(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	room := createTestRoom(t, app, ownerID, movieID)

	const guestCount = 2
	userIDs := []int64{ownerID}
	for i := 0; i < guestCount; i++ {
		guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
			Name:     fmt.Sprintf("Storm Guest %d", i),
			Email:    fmt.Sprintf("storm-guest-%d@example.com", i),
			Password: "hashed",
		})
		if err != nil {
			t.Fatalf("create guest %d: %v", i, err)
		}
		userIDs = append(userIDs, guest.ID)
	}
	addMembersToRoom(t, app, room.ID, userIDs...)

	server := setupWatchRoomWSTestServer(t, app)
	defer server.Close()

	conns := make([]*websocket.Conn, 0, len(userIDs))
	for _, userID := range userIDs {
		conn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, userID)
		defer conn.Close()
		_ = readUntilEventType(t, conn, "room_snapshot")
		conns = append(conns, conn)
	}

	// Every client fires a burst of playback events at the same time. The
	// hub must survive the storm without deadlocking and still deliver a
	// final, distinctive seek to every client.
	const eventsPerClient = 15
	var writers sync.WaitGroup
	for _, conn := range conns {
		writers.Add(1)
		go func(conn *websocket.Conn) {
			defer writers.Done()
			for i := 0; i < eventsPerClient; i++ {
				eventType := []string{"play", "pause", "seek"}[i%3]
				_ = conn.WriteJSON(map[string]any{
					"type":         eventType,
					"position_sec": float64(i),
				})
			}
		}(conn)
	}
	writers.Wait()

	if err := conns[0].WriteJSON(map[string]any{"type": "seek", "position_sec": 5000}); err != nil {
		t.Fatalf("final seek event: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for i, conn := range conns {
		found := false
		for time.Now().Before(deadline) {
			if err := conn.SetReadDeadline(deadline); err != nil {
				t.Fatalf("set read deadline: %v", err)
			}
			var event watchRoomWSTestEvent
			if err := conn.ReadJSON(&event); err != nil {
				t.Fatalf("client %d read during storm drain: %v", i, err)
			}
			if event.Type == "playback_changed" && event.Playback != nil &&
				event.Playback.PositionSec >= 5000 && event.Playback.PositionSec < 5001 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("client %d never received the final seek event", i)
		}
	}
}

func TestWatchRoomWebSocket_PlaybackStateSurvivesPartialDisconnectAndResetsWhenEmpty(t *testing.T) {
	app := setupTestApp(t)
	defer closeWatchRoomWSTestApp(t, app)

	ctx := context.Background()
	ownerID, movieID := createTestUserAndMovie(t, app)
	guest, err := app.Queries.CreateUser(ctx, database.CreateUserParams{
		Name:     "Lifecycle Guest",
		Email:    "lifecycle-guest@example.com",
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
	_ = readUntilEventType(t, ownerConn, "room_snapshot")

	if err := ownerConn.WriteJSON(map[string]any{"type": "play", "position_sec": 100}); err != nil {
		t.Fatalf("owner play event: %v", err)
	}
	_ = readUntilEventType(t, ownerConn, "playback_changed")

	// While the owner stays connected, a joining guest sees live playback.
	guestConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, guest.ID)
	guestSnapshot := readUntilEventType(t, guestConn, "room_snapshot")
	if guestSnapshot.Playback == nil || guestSnapshot.Playback.Paused {
		t.Fatalf("guest snapshot should reflect live playback, got %+v", guestSnapshot.Playback)
	}
	if guestSnapshot.Playback.PositionSec < 100 || guestSnapshot.Playback.PositionSec > 102 {
		t.Fatalf("guest snapshot position = %.3f, want ~100", guestSnapshot.Playback.PositionSec)
	}
	_ = guestConn.Close()

	// Wait for the guest departure to settle so the owner is last out.
	_ = readUntilEventType(t, ownerConn, "member_left")
	_ = ownerConn.Close()

	// Once the room fully empties, its in-memory session is discarded and
	// a fresh connection starts from the default paused state.
	var rejoinSnapshot watchRoomWSTestEvent
	for attempt := 0; attempt < 20; attempt++ {
		rejoinConn, _ := dialWatchRoomSocket(t, app, server.URL, room.ID, ownerID)
		rejoinSnapshot = readUntilEventType(t, rejoinConn, "room_snapshot")
		_ = rejoinConn.Close()
		if rejoinSnapshot.Playback != nil && rejoinSnapshot.Playback.Paused {
			break
		}
		// The previous socket's server-side cleanup may not have finished;
		// give it a moment and retry.
		time.Sleep(50 * time.Millisecond)
	}
	if rejoinSnapshot.Playback == nil || !rejoinSnapshot.Playback.Paused || rejoinSnapshot.Playback.PositionSec != 0 {
		t.Fatalf("expected fresh paused snapshot after room emptied, got %+v", rejoinSnapshot.Playback)
	}
}
