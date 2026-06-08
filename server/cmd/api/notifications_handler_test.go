package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func TestCreateNotification_HTTPCreatesMovieRequest(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	user := createTestUser(t, app, "Requester", "requester@example.com", false)

	router := chi.NewRouter()
	router.Post("/api/notifications", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user.ID)
		app.CreateNotification(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(`{
		"title": "movie_request",
		"message": "Requester: requester@example.com",
		"isAdmin": true
	}`))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error bool `json:"error"`
		Data  struct {
			Notification struct {
				ID              int64  `json:"id"`
				CreatedByUserID int64  `json:"created_by_user_id"`
				Title           string `json:"title"`
				Message         string `json:"message"`
				IsAdmin         bool   `json:"is_admin"`
			} `json:"notification"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error {
		t.Fatalf("expected success response, got %s", w.Body.String())
	}
	if resp.Data.Notification.Title != helpers.NOTIFICATION_TITLE_MOVIE_REQUEST {
		t.Fatalf("title = %q, want %q", resp.Data.Notification.Title, helpers.NOTIFICATION_TITLE_MOVIE_REQUEST)
	}
	if !resp.Data.Notification.IsAdmin {
		t.Fatal("expected notification to target admins")
	}
	if resp.Data.Notification.CreatedByUserID != user.ID {
		t.Fatalf("created_by_user_id = %d, want %d", resp.Data.Notification.CreatedByUserID, user.ID)
	}
}

func TestCreateNotification_HTTPRejectsInvalidTitle(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	user := createTestUser(t, app, "Requester", "requester@example.com", false)

	router := chi.NewRouter()
	router.Post("/api/notifications", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user.ID)
		app.CreateNotification(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(`{
		"title": "invalid",
		"message": "Requester: requester@example.com",
		"isAdmin": true
	}`))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}

func TestCreateNotification_HTTPRejectsEmptyMessage(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	user := createTestUser(t, app, "Requester", "requester@example.com", false)

	router := chi.NewRouter()
	router.Post("/api/notifications", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, user.ID)
		app.CreateNotification(w, r)
	})
	handler := app.SessionManager.LoadAndSave(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(`{
		"title": "movie_request",
		"message": "   ",
		"isAdmin": true
	}`))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}
}
