package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/scanner"

	"github.com/go-chi/chi/v5"
)

func TestShowScanAdminAuthorizationAndStatuses(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		authenticated, admin bool
		status               scanner.StartStatus
		want                 int
	}{
		{"unauthenticated", false, false, scanner.StartStarted, 401},
		{"non-admin", true, false, scanner.StartStarted, 403},
		{"started", true, true, scanner.StartStarted, 200},
		{"already running", true, true, scanner.StartAlreadyRunning, 409},
		{"unconfigured", true, true, scanner.StartNotConfigured, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := setupSettingsTestApp(t)
			defer app.DB.Close()
			var userID int64
			if tc.authenticated {
				user := createTestUser(t, app, "User", "tv@example.com", tc.admin)
				userID = user.ID
			}
			calls := 0
			app.ShowScanner = showStartFunc(func() scanner.StartResult { calls++; return scanner.StartResult{Status: tc.status} })
			router := chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if userID != 0 {
						app.SessionManager.Put(r.Context(), cookieUserID, userID)
					}
					next.ServeHTTP(w, r)
				})
			})
			router.Route("/api", app.registerSettingsRoutes)
			handler := app.SessionManager.LoadAndSave(router)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/settings/scan/shows", nil))
			if w.Code != tc.want {
				t.Fatalf("status %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			allowed := tc.authenticated && tc.admin
			if allowed && calls != 1 || !allowed && calls != 0 {
				t.Fatal("unexpected scanner invocation", calls)
			}
		})
	}
}
func TestStartupStartsShowScanner(t *testing.T) {
	for _, status := range []scanner.StartStatus{scanner.StartStarted, scanner.StartNotConfigured, scanner.StartAlreadyRunning} {
		app := setupTestApp(t)
		calls := 0
		app.ShowScanner = showStartFunc(func() scanner.StartResult { calls++; return scanner.StartResult{Status: status} })
		app.MovieScanner = movieStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartStarted} })
		app.MusicScanner = musicStartFunc(func() scanner.StartResult { return scanner.StartResult{Status: scanner.StartStarted} })
		startLibraryScansAtStartup(app)
		if calls != 1 {
			t.Fatal("startup did not invoke TV scanner")
		}
		app.DB.Close()
	}
}
