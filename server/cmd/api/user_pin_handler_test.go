package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/database"

	"github.com/go-chi/chi/v5"
)

func TestValidatePin(t *testing.T) {
	cases := []struct {
		name    string
		pin     string
		wantErr bool
	}{
		{"four digits", "1234", false},
		{"all zeros", "0000", false},
		{"too short", "123", true},
		{"too long", "12345", true},
		{"letter", "12a4", true},
		{"space", "12 4", true},
		{"empty", "", true},
		{"non-ascii digits", "١٢٣٤", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePin(tc.pin)
			if tc.wantErr && err == nil {
				t.Fatalf("validatePin(%q) = nil, want error", tc.pin)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validatePin(%q) = %v, want nil", tc.pin, err)
			}
		})
	}
}

// mountPinRouter serves the PIN routes behind a session-injecting middleware,
// mirroring mountPlaybackRouter. userID 0 leaves the request unauthenticated.
func mountPinRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID != 0 {
				app.SessionManager.Put(r.Context(), cookieUserID, userID)
			}
			next.ServeHTTP(w, r)
		})
	})
	r.Group(func(r chi.Router) {
		r.Use(app.IsAuth)
		r.Get("/api/user/pin", app.GetUserPin)
		r.Put("/api/user/pin", app.UpdateUserPin)
		r.Post("/api/user/pin/verify", app.VerifyUserPin)
	})

	return app.SessionManager.LoadAndSave(r)
}

func setupPinTestApp(t *testing.T) *Application {
	t.Helper()
	app := setupTestApp(t)
	app.InitSession()
	return app
}

func pinRequest(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func storedPin(t *testing.T, app *Application, userID int64) sql.NullString {
	t.Helper()
	user, err := app.Queries.GetUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	return user.Pin
}

type pinUserEnvelope struct {
	Data struct {
		User struct {
			HasPin bool `json:"has_pin"`
		} `json:"user"`
	} `json:"data"`
}

func decodeHasPin(t *testing.T, body []byte) bool {
	t.Helper()
	var env pinUserEnvelope
	err := json.Unmarshal(body, &env)
	if err != nil {
		t.Fatalf("decode user envelope: %v\nbody=%s", err, string(body))
	}
	return env.Data.User.HasPin
}

func TestUpdateUserPin_SetViaSession(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	w := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if !decodeHasPin(t, w.Body.Bytes()) {
		t.Fatalf("expected has_pin true in response: %s", w.Body.String())
	}

	pin := storedPin(t, app, user.ID)
	if !pin.Valid || pin.String != "1234" {
		t.Fatalf("expected stored pin 1234, got valid=%v %q", pin.Valid, pin.String)
	}

	if strings.Contains(w.Body.String(), `"pin":"1234"`) {
		t.Fatalf("plaintext pin leaked in user response: %s", w.Body.String())
	}
}

func TestUserPinHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Contract User", "contract@example.com", false)
	handler := mountPinRouter(app, user.ID)

	updateReq := newOpenAPIJSONRequest(http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	addOpenAPITestCookie(updateReq)
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, updateReq)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	assertOpenAPIExchange(t, "updateUserPin", updateReq, updateResponse)

	getReq := httptest.NewRequest(http.MethodGet, "/api/user/pin", nil)
	addOpenAPITestCookie(getReq)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getReq)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	assertOpenAPIExchange(t, "getUserPin", getReq, getResponse)

	verifyReq := newOpenAPIJSONRequest(http.MethodPost, "/api/user/pin/verify", `{"pin":"1234"}`)
	addOpenAPITestCookie(verifyReq)
	verifyResponse := httptest.NewRecorder()
	handler.ServeHTTP(verifyResponse, verifyReq)
	if verifyResponse.Code != http.StatusOK {
		t.Fatalf("verify status = %d, body = %s", verifyResponse.Code, verifyResponse.Body.String())
	}
	assertOpenAPIExchange(t, "verifyUserPin", verifyReq, verifyResponse)
}

func TestUpdateUserPin_SetViaDeviceToken(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()
	app.InitRouter()

	user := createTestUser(t, app, "TV Owner", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	req := httptest.NewRequest(http.MethodPut, "/api/user/pin", strings.NewReader(`{"pin":"4321"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	pin := storedPin(t, app, user.ID)
	if !pin.Valid || pin.String != "4321" {
		t.Fatalf("expected stored pin 4321, got valid=%v %q", pin.Valid, pin.String)
	}
}

func TestUpdateUserPin_InvalidFormatRejected(t *testing.T) {
	cases := []string{
		`{"pin":"123"}`,
		`{"pin":"12345"}`,
		`{"pin":"12a4"}`,
	}

	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	for _, body := range cases {
		w := pinRequest(t, handler, http.MethodPut, "/api/user/pin", body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: expected 400, got %d: %s", body, w.Code, w.Body.String())
		}
	}

	pin := storedPin(t, app, user.ID)
	if pin.Valid {
		t.Fatalf("expected pin unchanged (NULL), got %q", pin.String)
	}
}

func TestUpdateUserPin_ChangeRequiresCurrentPin(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	seed := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed: expected 200, got %d: %s", seed.Code, seed.Body.String())
	}

	missing := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"5678"}`)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing current_pin: expected 400, got %d: %s", missing.Code, missing.Body.String())
	}

	wrong := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"5678","current_pin":"0000"}`)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong current_pin: expected 401, got %d: %s", wrong.Code, wrong.Body.String())
	}

	correct := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"5678","current_pin":"1234"}`)
	if correct.Code != http.StatusOK {
		t.Fatalf("correct current_pin: expected 200, got %d: %s", correct.Code, correct.Body.String())
	}

	pin := storedPin(t, app, user.ID)
	if !pin.Valid || pin.String != "5678" {
		t.Fatalf("expected stored pin 5678, got valid=%v %q", pin.Valid, pin.String)
	}
}

func TestUpdateUserPin_RemoveClearsPin(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	seed := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed: expected 200, got %d: %s", seed.Code, seed.Body.String())
	}

	remove := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"","current_pin":"1234"}`)
	if remove.Code != http.StatusOK {
		t.Fatalf("remove: expected 200, got %d: %s", remove.Code, remove.Body.String())
	}
	if decodeHasPin(t, remove.Body.Bytes()) {
		t.Fatalf("expected has_pin false after removal: %s", remove.Body.String())
	}

	pin := storedPin(t, app, user.ID)
	if pin.Valid {
		t.Fatalf("expected pin NULL after removal, got %q", pin.String)
	}
}

func TestUpdateUserPin_RemoveWhenUnsetIsIdempotent(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	w := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":""}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserPin_SessionOnly(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	empty := pinRequest(t, handler, http.MethodGet, "/api/user/pin", "")
	if empty.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", empty.Code, empty.Body.String())
	}
	if !strings.Contains(empty.Body.String(), `"pin":null`) {
		t.Fatalf("expected pin null when unset, got: %s", empty.Body.String())
	}

	seed := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed: expected 200, got %d: %s", seed.Code, seed.Body.String())
	}

	get := pinRequest(t, handler, http.MethodGet, "/api/user/pin", "")
	if get.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", get.Code, get.Body.String())
	}
	if !strings.Contains(get.Body.String(), `"pin":"1234"`) {
		t.Fatalf("expected pin 1234 for session read, got: %s", get.Body.String())
	}
	cacheControl := get.Header().Get("Cache-Control")
	if cacheControl != "private, no-store" {
		t.Fatalf("expected private no-store cache policy, got %q", cacheControl)
	}
}

func TestGetUserPin_RejectsDeviceToken(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()
	app.InitRouter()

	user := createTestUser(t, app, "TV Owner", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	_, err := app.Queries.UpdateUserPin(context.Background(), database.UpdateUserPinParams{
		Pin: sql.NullString{String: "1234", Valid: true},
		ID:  user.ID,
	})
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/user/pin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("device token must not read the pin: expected 401, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "1234") {
		t.Fatalf("pin leaked to device token: %s", w.Body.String())
	}
}

func TestVerifyUserPin(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	noPin := pinRequest(t, handler, http.MethodPost, "/api/user/pin/verify", `{"pin":"1234"}`)
	if noPin.Code != http.StatusBadRequest {
		t.Fatalf("no pin set: expected 400, got %d: %s", noPin.Code, noPin.Body.String())
	}

	seed := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed: expected 200, got %d: %s", seed.Code, seed.Body.String())
	}

	malformed := pinRequest(t, handler, http.MethodPost, "/api/user/pin/verify", `{"pin":"12"}`)
	if malformed.Code != http.StatusBadRequest {
		t.Fatalf("malformed pin: expected 400, got %d: %s", malformed.Code, malformed.Body.String())
	}

	correct := pinRequest(t, handler, http.MethodPost, "/api/user/pin/verify", `{"pin":"1234"}`)
	if correct.Code != http.StatusOK {
		t.Fatalf("correct pin: expected 200, got %d: %s", correct.Code, correct.Body.String())
	}
	if !strings.Contains(correct.Body.String(), `"valid":true`) {
		t.Fatalf("expected valid true, got: %s", correct.Body.String())
	}

	wrong := pinRequest(t, handler, http.MethodPost, "/api/user/pin/verify", `{"pin":"0000"}`)
	if wrong.Code != http.StatusOK {
		t.Fatalf("wrong pin: expected 200, got %d: %s", wrong.Code, wrong.Body.String())
	}
	if !strings.Contains(wrong.Body.String(), `"valid":false`) {
		t.Fatalf("expected valid false, got: %s", wrong.Body.String())
	}
}

func TestVerifyUserPin_ViaDeviceToken(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()
	app.InitRouter()

	user := createTestUser(t, app, "TV Owner", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	_, err := app.Queries.UpdateUserPin(context.Background(), database.UpdateUserPinParams{
		Pin: sql.NullString{String: "1234", Valid: true},
		ID:  user.ID,
	})
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/user/pin/verify", strings.NewReader(`{"pin":"1234"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatalf("expected valid true, got: %s", w.Body.String())
	}
}

func TestPinAttempts_RateLimited(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()

	user := createTestUser(t, app, "Regular", "regular@example.com", false)
	handler := mountPinRouter(app, user.ID)

	seed := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"1234"}`)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed: expected 200, got %d: %s", seed.Code, seed.Body.String())
	}

	for i := 0; i < pinAttemptLimit; i++ {
		w := pinRequest(t, handler, http.MethodPost, "/api/user/pin/verify", `{"pin":"0000"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("attempt %d: expected 200, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	limited := pinRequest(t, handler, http.MethodPost, "/api/user/pin/verify", `{"pin":"0000"}`)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after %d attempts, got %d: %s", pinAttemptLimit, limited.Code, limited.Body.String())
	}

	// The bucket is shared with change/remove so the limiter cannot be
	// bypassed by guessing through the update endpoint instead.
	change := pinRequest(t, handler, http.MethodPut, "/api/user/pin", `{"pin":"5678","current_pin":"1234"}`)
	if change.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for change while limited, got %d: %s", change.Code, change.Body.String())
	}
}

func TestGetCurrentAuthUser_IncludesHasPin(t *testing.T) {
	app := setupPinTestApp(t)
	defer app.DB.Close()
	app.InitRouter()

	user := createTestUser(t, app, "TV Owner", "tv@example.com", false)
	token := createTestDevice(t, app, user.ID, "TV", "android_tv")

	fetchHasPin := func() (bool, string) {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/user", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		return decodeHasPin(t, w.Body.Bytes()), w.Body.String()
	}

	hasPin, body := fetchHasPin()
	if hasPin {
		t.Fatalf("expected has_pin false before setting, got: %s", body)
	}

	_, err := app.Queries.UpdateUserPin(context.Background(), database.UpdateUserPinParams{
		Pin: sql.NullString{String: "1234", Valid: true},
		ID:  user.ID,
	})
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}

	hasPin, body = fetchHasPin()
	if !hasPin {
		t.Fatalf("expected has_pin true after setting, got: %s", body)
	}
	if strings.Contains(body, "1234") {
		t.Fatalf("plaintext pin leaked in auth user response: %s", body)
	}
}
