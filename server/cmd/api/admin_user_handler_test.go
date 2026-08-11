package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
)

func mountAdminUserRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.AdminGetUsers(w, r)
	})
	r.Post("/api/admin/users", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.AdminCreateUser(w, r)
	})
	r.Patch("/api/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.AdminUpdateUser(w, r)
	})
	r.Delete("/api/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.AdminDeleteUser(w, r)
	})
	r.Put("/api/admin/users/{id}/password", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), cookieUserID, userID)
		app.AdminResetUserPassword(w, r)
	})

	return app.SessionManager.LoadAndSave(r)
}

func TestAdminUserListCreateAndPasswordReset_ConformToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := mountAdminUserRouter(app, admin.ID)

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	addOpenAPITestCookie(listReq)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listReq)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	assertOpenAPIExchange(t, "adminGetUsers", listReq, listResponse)

	createBody := `{"name":"New User","email":"new-user@example.com","password":"new password","is_admin":false}`
	createReq := newOpenAPIJSONRequest(http.MethodPost, "/api/admin/users", createBody)
	addOpenAPITestCookie(createReq)
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createReq)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createResponse.Code, createResponse.Body.String())
	}
	assertOpenAPIExchange(t, "adminCreateUser", createReq, createResponse)

	created, err := app.Queries.GetUserByEmail(context.Background(), "new-user@example.com")
	if err != nil {
		t.Fatalf("get created user: %v", err)
	}
	passwordBody := `{"password":"replacement password"}`
	passwordReq := newOpenAPIJSONRequest(http.MethodPut, "/api/admin/users/"+strconv.FormatInt(created.ID, 10)+"/password", passwordBody)
	addOpenAPITestCookie(passwordReq)
	passwordResponse := httptest.NewRecorder()
	handler.ServeHTTP(passwordResponse, passwordReq)
	if passwordResponse.Code != http.StatusOK {
		t.Fatalf("password reset status = %d, body = %s", passwordResponse.Code, passwordResponse.Body.String())
	}
	assertOpenAPIExchange(t, "adminResetUserPassword", passwordReq, passwordResponse)
}

func TestAdminUpdateUser_RejectsDemotingLastAdmin(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	target := createTestUser(t, app, "Solo Admin", "solo-admin@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	body := `{"name":"Solo Admin","email":"solo-admin@example.com","is_admin":false}`
	req := newOpenAPIJSONRequest(http.MethodPatch, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), body)
	addOpenAPITestCookie(req)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	user, err := app.Queries.GetUser(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("GetUser after rejected demotion: %v", err)
	}
	if !user.IsAdmin {
		t.Fatal("expected target user to remain an admin")
	}
}

func TestAdminUpdateUser_DemotesAdminWhenAnotherAdminExists(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	target := createTestUser(t, app, "Admin One", "admin-one@example.com", true)
	createTestUser(t, app, "Admin Two", "admin-two@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	body := `{"name":"Admin One","email":"admin-one@example.com","is_admin":false}`
	req := newOpenAPIJSONRequest(http.MethodPatch, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), body)
	addOpenAPITestCookie(req)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "adminUpdateUser", req, w)

	user, err := app.Queries.GetUser(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("GetUser after demotion: %v", err)
	}
	if user.IsAdmin {
		t.Fatal("expected target user to be demoted")
	}

	count, err := app.Queries.CountAdmins(context.Background())
	if err != nil {
		t.Fatalf("CountAdmins after demotion: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining admin, got %d", count)
	}
}

func TestAdminDeleteUser_RejectsDeletingLastAdmin(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	target := createTestUser(t, app, "Only Admin", "only-admin@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), nil)
	addOpenAPITestCookie(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	_, err := app.Queries.GetUser(context.Background(), target.ID)
	if err != nil {
		t.Fatalf("GetUser after rejected delete: %v", err)
	}
}

func TestAdminDeleteUser_DeletesAdminWhenAnotherAdminExists(t *testing.T) {
	app := setupSessionTestApp(t)
	defer app.DB.Close()

	target := createTestUser(t, app, "Delete Me", "delete-me@example.com", true)
	createTestUser(t, app, "Keep Me", "keep-me@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), nil)
	addOpenAPITestCookie(req)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	assertOpenAPIExchange(t, "adminDeleteUser", req, w)

	_, err := app.Queries.GetUser(context.Background(), target.ID)
	if err != sql.ErrNoRows {
		t.Fatalf("expected deleted user lookup to return sql.ErrNoRows, got %v", err)
	}

	count, err := app.Queries.CountAdmins(context.Background())
	if err != nil {
		t.Fatalf("CountAdmins after delete: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining admin, got %d", count)
	}
}
