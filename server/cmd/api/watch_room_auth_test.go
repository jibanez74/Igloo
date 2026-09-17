package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"igloo/cmd/internal/database"
)

type watchRoomAuthLoadResult struct {
	room database.WatchRoom
	err  error
}

func TestWatchRoomAuthCacheInvalidationPreventsLateFill(t *testing.T) {
	auth := newWatchRoomAuthCache()
	lookupStarted := make(chan struct{})
	releaseLookup := make(chan struct{})
	result := make(chan watchRoomAuthLoadResult, 1)
	staleRoom := database.WatchRoom{ID: 41}

	go func() {
		room, err := auth.load(context.Background(), staleRoom.ID, 7, func(context.Context, int64, int64) (database.WatchRoom, error) {
			close(lookupStarted)
			<-releaseLookup
			return staleRoom, nil
		})
		result <- watchRoomAuthLoadResult{room: room, err: err}
	}()

	<-lookupStarted
	auth.invalidateRoom(staleRoom.ID)
	close(releaseLookup)

	loaded := <-result
	if loaded.err != nil {
		t.Fatalf("in-flight load failed: %v", loaded.err)
	}
	if loaded.room.ID != staleRoom.ID {
		t.Fatalf("in-flight room ID = %d, want %d", loaded.room.ID, staleRoom.ID)
	}

	lookupCalls := 0
	_, err := auth.load(context.Background(), staleRoom.ID, 7, func(context.Context, int64, int64) (database.WatchRoom, error) {
		lookupCalls++
		return database.WatchRoom{}, sql.ErrNoRows
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("load after invalidation error = %v, want sql.ErrNoRows", err)
	}
	if lookupCalls != 1 {
		t.Fatalf("database lookup calls = %d, want 1; stale room was cached", lookupCalls)
	}
}

func TestWatchRoomAuthCacheFillAfterInvalidationIsCached(t *testing.T) {
	auth := newWatchRoomAuthCache()
	auth.invalidateRoom(52)

	lookupCalls := 0
	want := database.WatchRoom{ID: 52, OwnerUserID: 8}
	lookup := func(context.Context, int64, int64) (database.WatchRoom, error) {
		lookupCalls++
		return want, nil
	}

	first, err := auth.load(context.Background(), want.ID, want.OwnerUserID, lookup)
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	second, err := auth.load(context.Background(), want.ID, want.OwnerUserID, lookup)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}
	if first.ID != want.ID || second.ID != want.ID {
		t.Fatalf("loaded room IDs = %d and %d, want %d", first.ID, second.ID, want.ID)
	}
	if lookupCalls != 1 {
		t.Fatalf("database lookup calls = %d, want 1", lookupCalls)
	}
}

func TestWatchRoomAuthCacheInvalidationIsScopedToRoom(t *testing.T) {
	auth := newWatchRoomAuthCache()
	roomOne := database.WatchRoom{ID: 63}
	roomTwo := database.WatchRoom{ID: 64}

	seed := func(room database.WatchRoom, userID int64) {
		t.Helper()
		_, err := auth.load(context.Background(), room.ID, userID, func(context.Context, int64, int64) (database.WatchRoom, error) {
			return room, nil
		})
		if err != nil {
			t.Fatalf("seed room %d user %d: %v", room.ID, userID, err)
		}
	}
	seed(roomOne, 10)
	seed(roomOne, 11)
	seed(roomTwo, 12)

	auth.invalidateRoom(roomOne.ID)

	roomOneLookups := 0
	for _, userID := range []int64{10, 11} {
		_, err := auth.load(context.Background(), roomOne.ID, userID, func(context.Context, int64, int64) (database.WatchRoom, error) {
			roomOneLookups++
			return database.WatchRoom{}, sql.ErrNoRows
		})
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("room one user %d error = %v, want sql.ErrNoRows", userID, err)
		}
	}
	if roomOneLookups != 2 {
		t.Fatalf("room one database lookups = %d, want 2", roomOneLookups)
	}

	unrelatedLookup := errors.New("unrelated room cache miss")
	loaded, err := auth.load(context.Background(), roomTwo.ID, 12, func(context.Context, int64, int64) (database.WatchRoom, error) {
		return database.WatchRoom{}, unrelatedLookup
	})
	if err != nil {
		t.Fatalf("unrelated room was invalidated: %v", err)
	}
	if loaded.ID != roomTwo.ID {
		t.Fatalf("unrelated room ID = %d, want %d", loaded.ID, roomTwo.ID)
	}
}

func TestWatchRoomAuthCacheDoesNotCacheLookupErrors(t *testing.T) {
	auth := newWatchRoomAuthCache()
	wantErr := errors.New("query failed")
	lookupCalls := 0

	for range 2 {
		_, err := auth.load(context.Background(), 75, 13, func(context.Context, int64, int64) (database.WatchRoom, error) {
			lookupCalls++
			return database.WatchRoom{}, wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("load error = %v, want %v", err, wantErr)
		}
	}

	if lookupCalls != 2 {
		t.Fatalf("database lookup calls = %d, want 2", lookupCalls)
	}
}

func TestWatchRoomAuthCacheSuccessfulLookupExpiresAfterThirtySeconds(t *testing.T) {
	auth := newWatchRoomAuthCache()
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	lookupCalls := 0
	lookup := func(context.Context, int64, int64) (database.WatchRoom, error) {
		lookupCalls++
		return database.WatchRoom{ID: 86}, nil
	}

	_, err := auth.load(context.Background(), 86, 14, lookup)
	if err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	now = now.Add(watchRoomAuthCacheTTL - time.Nanosecond)
	_, err = auth.load(context.Background(), 86, 14, lookup)
	if err != nil {
		t.Fatalf("load before expiration failed: %v", err)
	}
	if lookupCalls != 1 {
		t.Fatalf("lookup calls before expiration = %d, want 1", lookupCalls)
	}

	now = now.Add(time.Nanosecond)
	_, err = auth.load(context.Background(), 86, 14, lookup)
	if err != nil {
		t.Fatalf("load at expiration failed: %v", err)
	}
	if lookupCalls != 2 {
		t.Fatalf("lookup calls at expiration = %d, want 2", lookupCalls)
	}
}
