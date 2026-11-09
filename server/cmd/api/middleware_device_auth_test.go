package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// /auth/user carries DeviceTokenAuth alone and answers 401 itself; the
// protected group adds IsAuth. Both must refuse a token no device owns.
func TestDeviceTokenAuth_RejectsUnknownToken(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	for _, path := range []string{"/api/auth/user", "/api/devices/"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+deviceTokenPrefix+"not-a-real-token")
			w := httptest.NewRecorder()
			app.Router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestDeviceTokenAuth_IgnoresNonDeviceBearerTokens(t *testing.T) {
	app := setupSessionTestApp(t)
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

func TestDeviceTokenAuth_StaleTokenAllowsQuickConnectRecovery(t *testing.T) {
	app := setupSessionTestApp(t)
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
	app := setupSessionTestApp(t)
	app.InitRouter()

	createTestUser(t, app, "Login User", "login@example.com", false)

	stale := "Bearer " + deviceTokenPrefix + "stale-token"

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Authorization", stale)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	body := fmt.Sprintf(`{"email":"login@example.com","password":%q}`, testUserPassword)
	req = newOpenAPIJSONRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Authorization", stale)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "authenticateUser", req, w)
}

func TestDeviceTokenAuth_WebSocketRouteRejectsUnknownToken(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/watch-rooms/1/ws", nil)
	req.Header.Set("Authorization", "Bearer "+deviceTokenPrefix+"stale-token")
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

// Admin routes take a session cookie only. Any device token gets 401 there,
// an admin's included, even when a valid admin cookie rides along, so a
// stolen TV token cannot manage users or reset the admin's own password.
func TestDeviceTokenAuth_DeviceTokensCannotUseAdminRoutes(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	regular := createTestUser(t, app, "Regular", "regular@example.com", false)
	adminToken := createTestDevice(t, app, admin.ID, "TV", "android_tv")
	regularToken := createTestDevice(t, app, regular.ID, "Phone", "android")
	adminCookie := newAuthSessionCookie(t, app, admin.ID)

	before, err := app.Queries.GetUser(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("load admin: %v", err)
	}

	requests := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodGet, "/api/admin/users", ""},
		{http.MethodPut, fmt.Sprintf("/api/admin/users/%d/password", admin.ID), `{"password":"taken-over-password"}`},
		{http.MethodGet, "/api/settings/general", ""},
		{http.MethodPost, "/api/settings/scan/movies", ""},
	}
	callers := []struct {
		name       string
		token      string
		withCookie bool
	}{
		{"admin device", adminToken, false},
		{"admin device with admin cookie", adminToken, true},
		{"regular device", regularToken, false},
	}

	for _, caller := range callers {
		for _, request := range requests {
			t.Run(caller.name+" "+request.method+" "+request.target, func(t *testing.T) {
				req := httptest.NewRequest(request.method, request.target, strings.NewReader(request.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+caller.token)
				if caller.withCookie {
					req.AddCookie(adminCookie)
				}
				w := httptest.NewRecorder()
				app.Router.ServeHTTP(w, req)

				if w.Code != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
				}
			})
		}
	}

	after, err := app.Queries.GetUser(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("reload admin: %v", err)
	}
	if after.Password != before.Password {
		t.Fatal("a device token reset the admin's password")
	}

	cookieOnly := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	cookieOnly.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, cookieOnly)
	if w.Code != http.StatusOK {
		t.Fatalf("admin cookie: status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	deviceRoute := httptest.NewRequest(http.MethodGet, "/api/continue-watching", nil)
	deviceRoute.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, deviceRoute)
	if w.Code != http.StatusOK {
		t.Fatalf("admin device on a viewer route: status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
}

func TestDeviceTokenAuth_ThrottlesLastUsedWrites(t *testing.T) {
	app := setupSessionTestApp(t)
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
