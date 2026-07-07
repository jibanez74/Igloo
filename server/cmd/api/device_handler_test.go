package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"
)

// createTestDevice inserts a device row directly and returns its bearer token.
func createTestDevice(t *testing.T, app *Application, userID int64, name, platform string) string {
	t.Helper()

	token, tokenHash, err := generateDeviceToken()
	if err != nil {
		t.Fatalf("generate device token: %v", err)
	}

	_, err = app.Queries.CreateDevice(context.Background(), database.CreateDeviceParams{
		UserID:    userID,
		Name:      name,
		Platform:  platform,
		TokenHash: tokenHash,
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	return token
}

func createTestUserWithPassword(t *testing.T, app *Application, name, email, password string) database.User {
	t.Helper()

	hashed, err := helpers.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     name,
		Email:    email,
		Password: hashed,
		IsAdmin:  false,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create user %q: %v", email, err)
	}
	return user
}

type deviceListResponse struct {
	Error bool `json:"error"`
	Data  struct {
		Devices []struct {
			ID         int64   `json:"id"`
			Name       string  `json:"name"`
			Platform   string  `json:"platform"`
			AppVersion *string `json:"app_version"`
			IsCurrent  bool    `json:"is_current"`
		} `json:"devices"`
	} `json:"data"`
}

func TestAuthenticateDevice_IssuesToken(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	createTestUserWithPassword(t, app, "Device User", "device@example.com", "correct horse")

	body := `{"email":"device@example.com","password":"correct horse","device_name":"Pixel","platform":"android"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", strings.NewReader(body))
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

	// The device login must not create a session cookie.
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == app.SessionManager.Cookie.Name && cookie.Value != "" {
			t.Fatalf("device login set a session cookie: %v", cookie)
		}
	}
}

func TestAuthenticateDevice_RejectsWrongPassword(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	createTestUserWithPassword(t, app, "Device User", "device@example.com", "correct horse")

	body := `{"email":"device@example.com","password":"wrong","device_name":"Pixel"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", strings.NewReader(body))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestAuthenticateDevice_IsRateLimited(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	body := `{"email":"nobody@example.com","password":"wrong","device_name":"Pixel"}`
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", strings.NewReader(body))
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i+1, w.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/device-login", strings.NewReader(body))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("11th attempt status = %d, want 429, body = %s", w.Code, w.Body.String())
	}
}

func TestGetDevices_ListsOwnDevicesAndMarksCurrent(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	other := createTestUser(t, app, "Other", "other@example.com", false)
	token := createTestDevice(t, app, user.ID, "Living Room TV", "android_tv")
	createTestDevice(t, app, other.ID, "Other Phone", "ios")

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	var resp deviceListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Fatalf("decode devices response: %v", err)
	}

	if len(resp.Data.Devices) != 1 {
		t.Fatalf("devices = %d, want 1 (only own devices)", len(resp.Data.Devices))
	}
	if resp.Data.Devices[0].Name != "Living Room TV" || !resp.Data.Devices[0].IsCurrent {
		t.Fatalf("device = %+v, want own device marked current", resp.Data.Devices[0])
	}

	if strings.Contains(w.Body.String(), "token_hash") {
		t.Fatal("device list response leaked token_hash")
	}
}

func TestRevokeDevice_InvalidatesToken(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	token := createTestDevice(t, app, user.ID, "Old Tablet", "android")

	device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup device: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/devices/%d", device.ID), nil)
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	// The revoked token no longer authenticates.
	req = httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestRevokeDevice_CannotRevokeOtherUsersDevice(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	owner := createTestUser(t, app, "Owner", "owner@example.com", false)
	attacker := createTestUser(t, app, "Attacker", "attacker@example.com", false)
	token := createTestDevice(t, app, owner.ID, "TV", "android_tv")

	device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup device: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/devices/%d", device.ID), nil)
	req.AddCookie(newAuthSessionCookie(t, app, attacker.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-user revoke status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}

func TestRenameDevice_CannotRenameOtherUsersDevice(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	owner := createTestUser(t, app, "Owner", "owner@example.com", false)
	attacker := createTestUser(t, app, "Attacker", "attacker@example.com", false)
	token := createTestDevice(t, app, owner.ID, "TV", "android_tv")

	device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup device: %v", err)
	}

	body := `{"name":"Hacked"}`
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/devices/%d", device.ID), strings.NewReader(body))
	req.AddCookie(newAuthSessionCookie(t, app, attacker.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-user rename status = %d, want 404, body = %s", w.Code, w.Body.String())
	}

	unchanged, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup device after rename attempt: %v", err)
	}
	if unchanged.Name != "TV" {
		t.Fatalf("name = %q, want unchanged %q", unchanged.Name, "TV")
	}
}

func TestRenameDevice_RenamesOwnDevice(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Owner", "owner@example.com", false)
	token := createTestDevice(t, app, user.ID, "Pixel", "android")

	device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup device: %v", err)
	}

	body := `{"name":"Bedroom Phone"}`
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/devices/%d", device.ID), strings.NewReader(body))
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("rename status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	renamed, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("lookup renamed device: %v", err)
	}
	if renamed.Name != "Bedroom Phone" {
		t.Fatalf("name = %q, want %q", renamed.Name, "Bedroom Phone")
	}
}
