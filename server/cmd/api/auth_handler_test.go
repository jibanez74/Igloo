package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func newAuthSessionCookie(t *testing.T, app *Application, userID int64) *http.Cookie {
	t.Helper()

	ctx, err := app.SessionManager.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("load test session: %v", err)
	}

	app.SessionManager.Put(ctx, cookieUserID, userID)
	token, _, err := app.SessionManager.Commit(ctx)
	if err != nil {
		t.Fatalf("commit test session: %v", err)
	}

	return &http.Cookie{
		Name:  app.SessionManager.Cookie.Name,
		Value: token,
	}
}

type authUserResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    struct {
		User struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Email   string `json:"email"`
			IsAdmin bool   `json:"is_admin"`
		} `json:"user"`
	} `json:"data"`
}

func decodeAuthUserResponse(t *testing.T, w *httptest.ResponseRecorder) authUserResponse {
	t.Helper()

	var resp authUserResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode auth user response: %v\nbody=%s", err, w.Body.String())
	}
	return resp
}

func TestGetCurrentAuthUser_HTTPReturnsUnauthorizedWhenUnauthenticated(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}

	resp := decodeAuthUserResponse(t, w)
	if !resp.Error || resp.Message != notAuthorizedMessage {
		t.Fatalf("response = %+v, want not authorized error", resp)
	}
}

func TestGetCurrentAuthUser_HTTPReturnsCurrentUser(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Requester", "requester@example.com", false)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "getCurrentAuthUser", req, w)

	resp := decodeAuthUserResponse(t, w)
	if resp.Error {
		t.Fatalf("expected success response: %+v", resp)
	}
	if resp.Data.User.ID != user.ID || resp.Data.User.Email != user.Email {
		t.Fatalf("user = %+v, want id=%d email=%q", resp.Data.User, user.ID, user.Email)
	}
}

func TestGetCurrentAuthUser_HTTPReturnsUnauthorizedForStaleSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.AddCookie(newAuthSessionCookie(t, app, 999))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}

	resp := decodeAuthUserResponse(t, w)
	if !resp.Error || resp.Message != notAuthorizedMessage {
		t.Fatalf("response = %+v, want not authorized error", resp)
	}
}

func TestLogout_DeviceTokenRevokesDevice(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "TV User", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	device, err := app.Queries.GetDeviceByTokenHash(context.Background(), hashDeviceToken(token))
	if err != nil {
		t.Fatalf("get device by token hash: %v", err)
	}
	lastSeenKey := strconv.FormatInt(device.ID, 10)
	app.DeviceLastSeen.SetDefault(lastSeenKey, struct{}{})

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "destroySession", req, w)

	if _, found := app.DeviceLastSeen.Get(lastSeenKey); found {
		t.Fatal("DeviceLastSeen cache entry should be evicted after device logout")
	}

	// The token must be dead after logout.
	req = httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestLogout_SessionCookieDoesNotRevokeDevices(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	user := createTestUser(t, app, "Web User", "web@example.com", false)
	token := createTestDevice(t, app, user.ID, "Phone", "android")

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/logout", nil)
	req.AddCookie(newAuthSessionCookie(t, app, user.ID))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	// A browser logout must not revoke the user's paired devices.
	req = httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("device token status after cookie logout = %d, want 200, body = %s", w.Code, w.Body.String())
	}
}
