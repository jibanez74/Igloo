package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Every route under registerAuthenticatedAPIRoutes sits behind IsAuth, so a
// request without a session or device token is refused before any handler
// runs. A few routes stand in for the group; the per-feature files no longer
// repeat this check.
func TestIsAuth_RejectsRequestsWithoutASession(t *testing.T) {
	app := setupSessionTestApp(t)
	handler := authenticatedRouter(t, app, 0)

	requests := []struct {
		method string
		target string
	}{
		{http.MethodGet, "/api/watch-rooms"},
		{http.MethodGet, "/api/shows/1"},
		{http.MethodGet, "/api/shows/1/seasons/1/episodes"},
		{http.MethodGet, "/api/settings/playback"},
		{http.MethodPut, "/api/settings/playback"},
	}
	for _, tc := range requests {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(tc.method, tc.target, nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", w.Code, w.Body.String())
			}
		})
	}
}

// A panicking handler must answer 500 and leave the server running; the
// router installs chi's Recoverer for that before any route.
func TestRouter_RecoversFromHandlerPanics(t *testing.T) {
	app := setupSessionTestApp(t)
	app.InitRouter()
	app.Router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("handler bug")
	})

	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}
