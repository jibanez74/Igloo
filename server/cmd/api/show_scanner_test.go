package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"igloo/cmd/internal/scanner/movie"
	"igloo/cmd/internal/scanner/music"
	"igloo/cmd/internal/scanner/show"

	"github.com/go-chi/chi/v5"
)

type showStartFunc func() show.StartResult

func (f showStartFunc) Start() show.StartResult { return f() }

func TestShowScanAdminAuthorizationAndStatuses(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		authenticated, admin bool
		status               show.StartStatus
		want                 int
	}{
		{"unauthenticated", false, false, show.StartStarted, 401},
		{"non-admin", true, false, show.StartStarted, 403},
		{"started", true, true, show.StartStarted, 200},
		{"already running", true, true, show.StartAlreadyRunning, 409},
		{"unconfigured", true, true, show.StartNotConfigured, 500},
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
			app.ShowScanner = showStartFunc(func() show.StartResult { calls++; return show.StartResult{Status: tc.status} })
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
	for _, status := range []show.StartStatus{show.StartStarted, show.StartNotConfigured, show.StartAlreadyRunning} {
		app := setupTestApp(t)
		calls := 0
		app.ShowScanner = showStartFunc(func() show.StartResult { calls++; return show.StartResult{Status: status} })
		app.MovieScanner = movieStartFunc(func() movie.StartResult { return movie.StartResult{Status: movie.StartStarted} })
		app.MusicScanner = musicStartFunc(func() music.StartResult { return music.StartResult{Status: music.StartStarted} })
		startLibraryScansAtStartup(app)
		if calls != 1 {
			t.Fatal("startup did not invoke TV scanner")
		}
		app.DB.Close()
	}
}
