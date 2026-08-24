package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestDeviceTokenAuth_StaleTokenAllowsDeviceLogin(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	createTestUserWithPassword(t, app, "Revoked TV", "revoked@example.com", "correct horse")

	// A client whose token was revoked still sends it; device login must
	// authenticate the credentials instead of rejecting the stale token.
	body := `{"email":"revoked@example.com","password":"correct horse","device_name":"TV","platform":"android_tv"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+deviceTokenPrefix+"stale-token")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	var resp quickConnectRedeemResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode device login response: %v", err)
	}
	if !strings.HasPrefix(resp.Data.Token, deviceTokenPrefix) {
		t.Fatalf("token = %q, want %q prefix", resp.Data.Token, deviceTokenPrefix)
	}
}

func TestDeviceTokenAuth_StaleTokenAllowsQuickConnectRecovery(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	stale := "Bearer " + deviceTokenPrefix + "stale-token"

	body := `{"device_name":"Living Room TV","platform":"android_tv","app_version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/quick-connect/initiate", strings.NewReader(body))
	req.Header.Set("Authorization", stale)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("initiate status = %d, want 201, body = %s", w.Code, w.Body.String())
	}

	// Redeem with an unknown code reaches its normal validation (404), not a
	// 401 from the stale bearer token.
	req = httptest.NewRequest(http.MethodPost, "/api/quick-connect/redeem", strings.NewReader(`{"code":"XXXXXX","secret":"nope"}`))
	req.Header.Set("Authorization", stale)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("redeem status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestDeviceTokenAuth_StaleTokenAllowsPublicRoutes(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	createTestUserWithPassword(t, app, "Login User", "login@example.com", "correct horse")

	stale := "Bearer " + deviceTokenPrefix + "stale-token"

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Authorization", stale)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	body := `{"email":"login@example.com","password":"correct horse"}`
	req = newOpenAPIJSONRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Authorization", stale)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "authenticateUser", req, w)
}

func TestDeviceTokenAuth_ProtectedRouteRejectsUnknownToken(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/devices/", nil)
	req.Header.Set("Authorization", "Bearer "+deviceTokenPrefix+"stale-token")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestDeviceTokenAuth_WebSocketRouteRejectsUnknownToken(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms/1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+deviceTokenPrefix+"stale-token")
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

	// Backdate last_used_at so the first request visibly bumps it. The
	// timestamp must stay inside the inactivity cutoff or the middleware
	// would revoke the device instead.
	backdated := time.Now().UTC().Add(-time.Hour).Format(sqliteTimeLayout)
	_, err = app.DB.Exec("UPDATE devices SET last_used_at = ? WHERE id = ?", backdated, device.ID)
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
	if first.LastUsedAt == backdated {
		t.Fatal("first request did not update last_used_at")
	}

	// Backdate again: within the throttle window no further write happens.
	_, err = app.DB.Exec("UPDATE devices SET last_used_at = ? WHERE id = ?", backdated, device.ID)
	if err != nil {
		t.Fatalf("backdate last_used_at: %v", err)
	}

	makeRequest()
	second, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup after second request: %v", err)
	}
	if second.LastUsedAt != backdated {
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
