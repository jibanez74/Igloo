package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// sessionProbe reports the status a library request gets with this cookie or
// bearer token, the cheapest route behind IsAuth.
func sessionProbe(t *testing.T, app *Application, cookie *http.Cookie, bearer string) int {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/movies/library", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w.Code
}

// serveWithCookie serves one JSON request with cookie and returns the
// response, from which a renewed session cookie can be read.
func serveWithCookie(t *testing.T, app *Application, cookie *http.Cookie, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := newOpenAPIJSONRequest(method, target, body)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	return w
}

func renewedSessionCookie(t *testing.T, app *Application, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == app.SessionManager.Cookie.Name && cookie.Value != "" {
			return cookie
		}
	}
	t.Fatalf("response set no session cookie: %v", w.Header())
	return nil
}

func TestDeletedUsersOtherSessionsAreSignedOut(t *testing.T) {
	t.Run("self delete", func(t *testing.T) {
		app := setupSessionTestApp(t)
		app.InitRouter()
		user := createTestUser(t, app, "Leaving", "leaving@example.com", false)
		deleting := newAuthSessionCookie(t, app, user.ID)
		other := newAuthSessionCookie(t, app, user.ID)
		if code := sessionProbe(t, app, other, ""); code != http.StatusOK {
			t.Fatalf("other session before delete = %d, want 200", code)
		}

		w := serveWithCookie(t, app, deleting, http.MethodDelete, "/api/user", "")
		if w.Code != http.StatusOK {
			t.Fatalf("delete status = %d: %s", w.Code, w.Body.String())
		}
		if code := sessionProbe(t, app, other, ""); code != http.StatusUnauthorized {
			t.Fatalf("other session after delete = %d, want 401", code)
		}
	})

	t.Run("admin delete", func(t *testing.T) {
		app := setupSessionTestApp(t)
		app.InitRouter()
		admin := createTestUser(t, app, "Admin", "deleting-admin@example.com", true)
		user := createTestUser(t, app, "Removed", "removed@example.com", false)
		session := newAuthSessionCookie(t, app, user.ID)
		if code := sessionProbe(t, app, session, ""); code != http.StatusOK {
			t.Fatalf("session before delete = %d, want 200", code)
		}

		w := serveAs(t, app, admin.ID, http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(user.ID, 10), "")
		if w.Code != http.StatusOK {
			t.Fatalf("admin delete status = %d: %s", w.Code, w.Body.String())
		}
		if code := sessionProbe(t, app, session, ""); code != http.StatusUnauthorized {
			t.Fatalf("session after admin delete = %d, want 401", code)
		}
	})
}

// A password change keeps only the session that made it; every other session
// and every device token of the user stops working.
func TestPasswordChangeSignsOutOtherSessionsAndDevices(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()
	user := createTestUser(t, app, "Changer", "changer@example.com", false)
	changing := newAuthSessionCookie(t, app, user.ID)
	other := newAuthSessionCookie(t, app, user.ID)
	token := createTestDevice(t, app, user.ID, "Living Room TV", "android_tv")
	if code := sessionProbe(t, app, nil, token); code != http.StatusOK {
		t.Fatalf("device before change = %d, want 200", code)
	}

	body := fmt.Sprintf(`{"current_password":%q,"new_password":"a brand new password"}`, testUserPassword)
	w := serveWithCookie(t, app, changing, http.MethodPut, "/api/user/password", body)
	if w.Code != http.StatusOK {
		t.Fatalf("change status = %d: %s", w.Code, w.Body.String())
	}
	renewed := renewedSessionCookie(t, app, w)

	if code := sessionProbe(t, app, renewed, ""); code != http.StatusOK {
		t.Fatalf("changing session after change = %d, want 200", code)
	}
	if code := sessionProbe(t, app, other, ""); code != http.StatusUnauthorized {
		t.Fatalf("other session after change = %d, want 401", code)
	}
	if code := sessionProbe(t, app, nil, token); code != http.StatusUnauthorized {
		t.Fatalf("device after change = %d, want 401", code)
	}
}

func TestAdminPasswordResetSignsOutTheUser(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()
	admin := createTestUser(t, app, "Admin", "reset-admin@example.com", true)
	user := createTestUser(t, app, "Reset", "reset@example.com", false)
	adminSession := newAuthSessionCookie(t, app, admin.ID)
	userSession := newAuthSessionCookie(t, app, user.ID)
	token := createTestDevice(t, app, user.ID, "Phone", "android")

	w := serveWithCookie(t, app, adminSession, http.MethodPut, "/api/admin/users/"+strconv.FormatInt(user.ID, 10)+"/password", `{"password":"a brand new password"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reset status = %d: %s", w.Code, w.Body.String())
	}
	if code := sessionProbe(t, app, userSession, ""); code != http.StatusUnauthorized {
		t.Fatalf("user session after reset = %d, want 401", code)
	}
	if code := sessionProbe(t, app, nil, token); code != http.StatusUnauthorized {
		t.Fatalf("user device after reset = %d, want 401", code)
	}
	if code := sessionProbe(t, app, adminSession, ""); code != http.StatusOK {
		t.Fatalf("admin session after reset = %d, want 200", code)
	}
}

// Browser and device logins accept the same credentials, so they share one
// per-address budget.
func TestLoginAttemptsShareOneBudget(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()
	user := createTestUser(t, app, "Guessed", "guessed@example.com", false)

	attempt := func(target, body string) int {
		req := newOpenAPIJSONRequest(http.MethodPost, target, body)
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		return w.Code
	}
	wrongLogin := fmt.Sprintf(`{"email":%q,"password":"wrong password"}`, user.Email)
	wrongDevice := fmt.Sprintf(`{"email":%q,"password":"wrong password","device_name":"TV"}`, user.Email)
	for i := range loginAttemptLimit {
		target, body := "/api/auth/login", wrongLogin
		if i%2 == 1 {
			target, body = "/api/auth/device-login", wrongDevice
		}
		if code := attempt(target, body); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d on %s = %d, want 401", i+1, target, code)
		}
	}

	req := newOpenAPIJSONRequest(http.MethodPost, "/api/auth/login", wrongLogin)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	assertOpenAPIResponse(t, "authenticateUser", req, w)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("login past the budget = %d, want 429", w.Code)
	}
	if code := attempt("/api/auth/device-login", wrongDevice); code != http.StatusTooManyRequests {
		t.Fatalf("device login past the budget = %d, want 429", code)
	}
}

func TestPasswordChangeCurrentPasswordChecksAreLimited(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Target", "target@example.com", false)
	handler := authenticatedRouter(t, app, user.ID)
	body := `{"current_password":"not the password","new_password":"a brand new password"}`

	for i := range loginAttemptLimit {
		w := serveRequest(t, handler, http.MethodPut, "/api/user/password", body)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want 401: %s", i+1, w.Code, w.Body.String())
		}
	}
	req := newOpenAPIJSONRequest(http.MethodPut, "/api/user/password", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "updateUserPassword", req, w)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt past the budget = %d, want 429", w.Code)
	}
}

// A stolen device token must not be able to delete the account or move it to
// another email or password.
func TestAccountChangesRejectDeviceTokens(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()
	user := createTestUser(t, app, "Owner", "owner-account@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	for _, tt := range []struct {
		operationID, method, target, body string
	}{
		{"deleteUserAccount", http.MethodDelete, "/api/user", ""},
		{"updateUserEmail", http.MethodPut, "/api/user/email", `{"email":"thief@example.com"}`},
		{"updateUserPassword", http.MethodPut, "/api/user/password", fmt.Sprintf(`{"current_password":%q,"new_password":"a brand new password"}`, testUserPassword)},
	} {
		req := newOpenAPIJSONRequest(tt.method, tt.target, tt.body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		// The contract requires the cookie, so only the response is checked.
		assertOpenAPIResponse(t, tt.operationID, req, w)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s with a device token = %d, want 401: %s", tt.operationID, w.Code, w.Body.String())
		}
	}
	if code := sessionProbe(t, app, nil, token); code != http.StatusOK {
		t.Fatalf("device after rejected changes = %d, want 200", code)
	}
}

// /api/auth/user is how the web client decides it is signed in, so a revoked
// session must fail there too.
func TestCurrentAuthUserRejectsARevokedSession(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()
	user := createTestUser(t, app, "Revoked", "revoked@example.com", false)
	session := newAuthSessionCookie(t, app, user.ID)

	_, err := app.changePassword(t.Context(), user.ID, "new hash")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
	req.AddCookie(session)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "getCurrentAuthUser", req, w)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session on /api/auth/user = %d, want 401", w.Code)
	}
}
