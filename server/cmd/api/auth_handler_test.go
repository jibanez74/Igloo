package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/helpers"
)

func newAuthSessionCookie(t *testing.T, app *Application, userID int64) *http.Cookie {
	t.Helper()

	ctx, err := app.SessionManager.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("load test session: %v", err)
	}

	app.SessionManager.Put(ctx, helpers.COOKIE_USER_ID, userID)
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

func TestGetCurrentAuthUser_HTTPReturnsOKErrorWhenUnauthenticated(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	resp := decodeAuthUserResponse(t, w)
	if !resp.Error || resp.Message != helpers.NOT_AUTHORIZED_MESSAGE {
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

	resp := decodeAuthUserResponse(t, w)
	if resp.Error {
		t.Fatalf("expected success response: %+v", resp)
	}
	if resp.Data.User.ID != user.ID || resp.Data.User.Email != user.Email {
		t.Fatalf("user = %+v, want id=%d email=%q", resp.Data.User, user.ID, user.Email)
	}
}

func TestGetCurrentAuthUser_HTTPReturnsOKErrorForStaleSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.AddCookie(newAuthSessionCookie(t, app, 999))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}

	resp := decodeAuthUserResponse(t, w)
	if !resp.Error || resp.Message != helpers.NOT_AUTHORIZED_MESSAGE {
		t.Fatalf("response = %+v, want not authorized error", resp)
	}
}
