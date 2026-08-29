package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

const watchRoomAuthCacheTTL = 30 * time.Second

type watchRoomAuthLookup func(context.Context, int64, int64) (database.WatchRoom, error)

type watchRoomAuthKey struct {
	roomID int64
	userID int64
}

type watchRoomAuthEntry struct {
	room      database.WatchRoom
	expiresAt time.Time
}

// watchRoomAuthCache keeps successful membership checks off SQLite on the
// room media hot path. The generation and entries share one mutex so an
// invalidation cannot race with publication from an older database lookup.
type watchRoomAuthCache struct {
	mu         sync.Mutex
	generation uint64
	entries    map[watchRoomAuthKey]watchRoomAuthEntry
	now        func() time.Time
	nextSweep  time.Time
}

func newWatchRoomAuthCache() *watchRoomAuthCache {
	return &watchRoomAuthCache{
		entries: make(map[watchRoomAuthKey]watchRoomAuthEntry),
		now:     time.Now,
	}
}

func (auth *watchRoomAuthCache) purgeExpiredLocked(now time.Time) {
	sweepDue := auth.nextSweep.IsZero() || !now.Before(auth.nextSweep)
	if !sweepDue {
		return
	}

	for key, entry := range auth.entries {
		if !now.Before(entry.expiresAt) {
			delete(auth.entries, key)
		}
	}
	auth.nextSweep = now.Add(watchRoomAuthCacheTTL)
}

func (auth *watchRoomAuthCache) load(
	ctx context.Context,
	roomID, userID int64,
	lookup watchRoomAuthLookup,
) (database.WatchRoom, error) {
	key := watchRoomAuthKey{roomID: roomID, userID: userID}

	auth.mu.Lock()
	now := auth.now()
	auth.purgeExpiredLocked(now)
	cached, hit := auth.entries[key]
	if hit && !now.Before(cached.expiresAt) {
		delete(auth.entries, key)
		hit = false
	}
	if hit {
		auth.mu.Unlock()
		return cached.room, nil
	}
	generation := auth.generation
	auth.mu.Unlock()

	room, err := lookup(ctx, roomID, userID)
	if err != nil {
		return database.WatchRoom{}, err
	}

	auth.mu.Lock()
	if auth.generation == generation {
		auth.entries[key] = watchRoomAuthEntry{
			room:      room,
			expiresAt: auth.now().Add(watchRoomAuthCacheTTL),
		}
	}
	auth.mu.Unlock()

	return room, nil
}

// invalidateRoom advances the generation before removing every cached member
// authorization for the room. Any lookup already in flight sees the older
// generation and cannot publish its result after this method returns.
func (auth *watchRoomAuthCache) invalidateRoom(roomID int64) {
	auth.mu.Lock()
	auth.generation++
	for key := range auth.entries {
		if key.roomID == roomID {
			delete(auth.entries, key)
		}
	}
	auth.mu.Unlock()
}

// loadAuthorizedWatchRoom returns the room only when the user is a member.
// This runs on every room media request -- each HLS segment, and each
// byte-range request a browser issues while seeking a direct-play file -- so
// it is a single joined query and successful results are briefly cached.
// sql.ErrNoRows means "no room or no access" and callers do not distinguish
// the two. Denials and query failures are never cached.
func (app *Application) loadAuthorizedWatchRoom(ctx context.Context, roomID, userID int64) (database.WatchRoom, error) {
	lookup := func(ctx context.Context, roomID, userID int64) (database.WatchRoom, error) {
		return app.Queries.GetWatchRoomForMember(ctx, database.GetWatchRoomForMemberParams{
			ID:     roomID,
			UserID: userID,
		})
	}

	return app.WatchRoomAuthCache.load(ctx, roomID, userID, lookup)
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

// deleteWatchRoom invalidates authorization immediately after the database
// deletion succeeds. Callers may then clean up HLS and WebSocket state without
// leaving a window where a stale authorization can be republished.
func (app *Application) deleteWatchRoom(ctx context.Context, roomID int64) error {
	err := app.Queries.DeleteWatchRoom(ctx, roomID)
	if err != nil {
		return err
	}

	app.WatchRoomAuthCache.invalidateRoom(roomID)
	return nil
}
