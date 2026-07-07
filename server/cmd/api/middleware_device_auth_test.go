package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeviceTokenAuth_RejectsUnknownToken(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+deviceTokenPrefix+"not-a-real-token")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestDeviceTokenAuth_IgnoresNonDeviceBearerTokens(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	// A bearer token without the device prefix is not ours to judge; the
	// request falls through to session auth and fails there instead.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer some-other-token")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestDeviceTokenAuth_NonAdminDeviceCannotUseAdminRoutes(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	token := createTestDevice(t, app, user.ID, "Phone", "android")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403, body = %s", w.Code, w.Body.String())
	}
}

func TestDeviceTokenAuth_ThrottlesLastUsedWrites(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup device: %v", err)
	}

	// Backdate last_used_at so the first request visibly bumps it.
	_, err = app.DB.Exec("UPDATE devices SET last_used_at = '2000-01-01 00:00:00' WHERE id = ?", device.ID)
	if err != nil {
		t.Fatalf("backdate last_used_at: %v", err)
	}

	makeRequest := func() {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("bearer request status = %d, want 200, body = %s", w.Code, w.Body.String())
		}
	}

	makeRequest()
	first, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup after first request: %v", err)
	}
	if first.LastUsedAt == "2000-01-01 00:00:00" {
		t.Fatal("first request did not update last_used_at")
	}

	// Backdate again: within the throttle window no further write happens.
	_, err = app.DB.Exec("UPDATE devices SET last_used_at = '2000-01-01 00:00:00' WHERE id = ?", device.ID)
	if err != nil {
		t.Fatalf("backdate last_used_at: %v", err)
	}

	makeRequest()
	second, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup after second request: %v", err)
	}
	if second.LastUsedAt != "2000-01-01 00:00:00" {
		t.Fatalf("second request wrote last_used_at = %q despite throttle", second.LastUsedAt)
	}
}

func TestRateLimiter_WindowRollsOver(t *testing.T) {
	limiter := newRateLimiter()

	current := time.Now()
	limiter.now = func() time.Time { return current }

	for i := 0; i < 3; i++ {
		if !limiter.Allow("key", 3, time.Minute) {
			t.Fatalf("attempt %d unexpectedly limited", i+1)
		}
	}
	if limiter.Allow("key", 3, time.Minute) {
		t.Fatal("4th attempt in window should be limited")
	}

	// A new window admits attempts again.
	current = current.Add(time.Minute + time.Second)
	if !limiter.Allow("key", 3, time.Minute) {
		t.Fatal("attempt after window rollover should be allowed")
	}
}

func TestRateLimiter_KeysAreIndependent(t *testing.T) {
	limiter := newRateLimiter()

	if !limiter.Allow("a", 1, time.Minute) {
		t.Fatal("first attempt for key a should be allowed")
	}
	if limiter.Allow("a", 1, time.Minute) {
		t.Fatal("second attempt for key a should be limited")
	}
	if !limiter.Allow("b", 1, time.Minute) {
		t.Fatal("key b should be unaffected by key a")
	}
}
