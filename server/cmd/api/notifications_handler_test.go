package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func TestCreateNotification_HTTPCreatesMovieRequest(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	user := createTestUser(t, app, "Requester", "requester@example.com", false)

	handler := notificationTestServer(app, user.ID)

	w := httptest.NewRecorder()
	req := newOpenAPIJSONRequest(http.MethodPost, "/api/notifications", `{
		"title": "movie_request",
		"message": "Requester: requester@example.com",
		"isAdmin": true
	}`)
	addOpenAPITestCookie(req)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "createNotification", req, w)

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
	if resp.Data.Notification.Title != notificationTitleMovieRequest {
		t.Fatalf("title = %q, want %q", resp.Data.Notification.Title, notificationTitleMovieRequest)
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

	handler := notificationTestServer(app, user.ID)

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

	handler := notificationTestServer(app, user.ID)

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

func TestCreateNotification_HTTPRejectsNonAdminWithoutTargetUser(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	user := createTestUser(t, app, "Requester", "requester@example.com", false)
	handler := notificationTestServer(app, user.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(`{
		"title": "movie_request",
		"message": "Requester: requester@example.com",
		"isAdmin": false
	}`))
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Error || resp.Message != "non-admin notifications require a target user" {
		t.Fatalf("response = %+v, want non-admin target-user error", resp)
	}
}

// notificationTestServer wires the notification routes behind a session that is
// always authenticated as userID, so handlers that read chi URL params and the
// session user work end to end.
func notificationTestServer(app *Application, userID int64) http.Handler {
	router := chi.NewRouter()

	withUser := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			app.SessionManager.Put(r.Context(), cookieUserID, userID)
			next.ServeHTTP(w, r)
		})
	}

	router.Group(func(r chi.Router) {
		r.Use(withUser)
		r.Use(app.IsAuth)
		r.Route("/api", func(r chi.Router) {
			app.registerNotificationRoutes(r)
		})
	})

	return app.SessionManager.LoadAndSave(router)
}

type notificationListResponse struct {
	Data struct {
		Notifications []struct {
			ID            int64  `json:"id"`
			Title         string `json:"title"`
			Message       string `json:"message"`
			IsAdmin       bool   `json:"is_admin"`
			IsRead        bool   `json:"is_read"`
			CreatedByName string `json:"created_by_name"`
		} `json:"notifications"`
		UnreadCount int64 `json:"unread_count"`
	} `json:"data"`
}

func seedAdminQueueNotification(t *testing.T, app *Application, requesterID int64, message string) database.Notification {
	t.Helper()
	n, err := app.Queries.CreateNotification(context.Background(), database.CreateNotificationParams{
		CreatedByUserID: requesterID,
		UserID:          sql.NullInt64{},
		Title:           notificationTitleMovieRequest,
		Message:         message,
		IsAdmin:         true,
	})
	if err != nil {
		t.Fatalf("failed to seed notification: %v", err)
	}
	return n
}

func TestListNotifications_AdminSeesQueueRequesterDoesNot(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	requester := createTestUser(t, app, "Requester", "requester@example.com", false)
	seedAdminQueueNotification(t, app, requester.ID, "Requester: Requester <requester@example.com>")

	// Admin sees the admin-queue notification, unread, attributed to the requester.
	w := httptest.NewRecorder()
	notificationTestServer(app, admin.ID).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/notifications/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var adminResp notificationListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &adminResp); err != nil {
		t.Fatalf("unmarshal admin response: %v", err)
	}
	if len(adminResp.Data.Notifications) != 1 {
		t.Fatalf("admin notifications = %d, want 1, body = %s", len(adminResp.Data.Notifications), w.Body.String())
	}
	if adminResp.Data.UnreadCount != 1 {
		t.Fatalf("admin unread_count = %d, want 1", adminResp.Data.UnreadCount)
	}
	if got := adminResp.Data.Notifications[0]; got.IsRead || got.CreatedByName != "Requester" {
		t.Fatalf("admin notification = %+v, want unread from Requester", got)
	}

	// The non-admin requester does not see the admin queue.
	w = httptest.NewRecorder()
	notificationTestServer(app, requester.ID).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/notifications/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("requester list status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	var requesterResp notificationListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &requesterResp); err != nil {
		t.Fatalf("unmarshal requester response: %v", err)
	}
	if len(requesterResp.Data.Notifications) != 0 || requesterResp.Data.UnreadCount != 0 {
		t.Fatalf("requester saw admin queue: %d notifications, unread %d", len(requesterResp.Data.Notifications), requesterResp.Data.UnreadCount)
	}
}

func TestListNotifications_AcceptsCanonicalNoTrailingSlash(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	requester := createTestUser(t, app, "Requester", "requester@example.com", false)
	seedAdminQueueNotification(t, app, requester.ID, "Requester: Requester")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	addOpenAPITestCookie(req)
	notificationTestServer(app, admin.ID).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "listNotifications", req, w)

	var resp notificationListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp.Data.Notifications) != 1 {
		t.Fatalf("notifications = %d, want 1, body = %s", len(resp.Data.Notifications), w.Body.String())
	}
}

func TestListNotifications_ReturnsUnauthorizedForStaleSession(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()
	app.InitRouter()

	cookie := newAuthSessionCookie(t, app, 999)
	req := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}

	var resp helpers.JSONResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Error || resp.Message != notAuthorizedMessage {
		t.Fatalf("response = %+v, want not authorized error", resp)
	}

	ctx, err := app.SessionManager.Load(context.Background(), cookie.Value)
	if err != nil {
		t.Fatalf("load session after stale request: %v", err)
	}
	if app.SessionManager.Exists(ctx, cookieUserID) {
		t.Fatal("stale session still has a user id after notification request")
	}
}

func TestMarkNotificationRead_DropsUnreadCount(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	requester := createTestUser(t, app, "Requester", "requester@example.com", false)
	n := seedAdminQueueNotification(t, app, requester.ID, "please add a movie")
	server := notificationTestServer(app, admin.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/"+strconv.FormatInt(n.ID, 10)+"/read", nil)
	addOpenAPITestCookie(req)
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark read status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "markNotificationRead", req, w)

	w = httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/notifications/", nil))
	var resp notificationListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.UnreadCount != 0 {
		t.Fatalf("unread_count = %d, want 0 after mark read", resp.Data.UnreadCount)
	}
	if len(resp.Data.Notifications) != 1 || !resp.Data.Notifications[0].IsRead {
		t.Fatalf("notification not marked read: %+v", resp.Data.Notifications)
	}
}

func TestMarkAllNotificationsRead_ZeroesUnread(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	requester := createTestUser(t, app, "Requester", "requester@example.com", false)
	seedAdminQueueNotification(t, app, requester.ID, "request one")
	seedAdminQueueNotification(t, app, requester.ID, "request two")
	server := notificationTestServer(app, admin.ID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil)
	addOpenAPITestCookie(req)
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mark all status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "markAllNotificationsRead", req, w)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/notifications/unread-count", nil)
	addOpenAPITestCookie(req)
	server.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "getUnreadNotificationCount", req, w)
	var resp struct {
		Data struct {
			UnreadCount int64 `json:"unread_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.UnreadCount != 0 {
		t.Fatalf("unread_count = %d, want 0 after mark all read", resp.Data.UnreadCount)
	}
}

func TestDeleteNotification_RemovesAndThen404(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()
	app.InitSession()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	requester := createTestUser(t, app, "Requester", "requester@example.com", false)
	n := seedAdminQueueNotification(t, app, requester.ID, "delete me")
	server := notificationTestServer(app, admin.ID)
	idPath := "/api/notifications/" + strconv.FormatInt(n.ID, 10)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, idPath, nil)
	addOpenAPITestCookie(req)
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200, body = %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "deleteNotification", req, w)

	// Deleting again is a 404 — the row is gone.
	w = httptest.NewRecorder()
	server.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, idPath, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404, body = %s", w.Code, w.Body.String())
	}
}
