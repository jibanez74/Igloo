package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setDeviceLastUsedForTest(t *testing.T, app *Application, token, lastUsedAt string) {
	t.Helper()

	res, err := app.DB.Exec("UPDATE devices SET last_used_at = ? WHERE token_hash = ?", lastUsedAt, hashDeviceToken(token))
	if err != nil {
		t.Fatalf("update last_used_at: %v", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("rows affected: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows affected = %d, want 1", rows)
	}
}

func TestDeviceTokenAuth_RejectsAndDeletesStaleDevice(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	token := createTestDevice(t, app, user.ID, "Attic TV", "android_tv")
	setDeviceLastUsedForTest(t, app, token, "2020-01-01 00:00:00")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("stale device status = %d, want 401, body = %s", w.Code, w.Body.String())
	}

	// The stale device row is revoked on the spot, not just rejected.
	_, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("lookup after stale rejection err = %v, want sql.ErrNoRows", err)
	}
}

func TestDeviceTokenAuth_DeviceInsideCutoffAuthenticates(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	token := createTestDevice(t, app, user.ID, "Bedroom TV", "android_tv")

	// A day inside the cutoff must still authenticate; guards against an
	// inverted staleness comparison.
	lastUsed := time.Now().UTC().Add(-deviceInactivityTTL + 24*time.Hour).Format(sqliteTimeLayout)
	setDeviceLastUsedForTest(t, app, token, lastUsed)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("almost-stale device status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
}

func TestSweepStaleDevices_RemovesOnlyStaleRows(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	staleToken := createTestDevice(t, app, user.ID, "Old TV", "android_tv")
	freshToken := createTestDevice(t, app, user.ID, "New Phone", "ios")
	setDeviceLastUsedForTest(t, app, staleToken, "2020-01-01 00:00:00")

	app.sweepStaleDevices(context.Background())

	_, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(staleToken))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("stale device lookup err = %v, want sql.ErrNoRows", err)
	}

	fresh, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(freshToken))
	if err != nil {
		t.Fatalf("fresh device lookup err = %v, want it to survive the sweep", err)
	}
	if fresh.Name != "New Phone" {
		t.Fatalf("fresh device = %+v, want New Phone", fresh)
	}
}
