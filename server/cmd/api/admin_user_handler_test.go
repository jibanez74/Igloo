package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestAdminUserListCreateAndPasswordReset_ConformToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)

	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	handler := authenticatedRouter(t, app, admin.ID)

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listReq)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	assertOpenAPIExchange(t, "adminGetUsers", listReq, listResponse)

	createBody := `{"name":"New User","email":"new-user@example.com","password":"new password","is_admin":false}`
	createReq := newOpenAPIJSONRequest(http.MethodPost, "/api/admin/users", createBody)
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
	passwordResponse := httptest.NewRecorder()
	handler.ServeHTTP(passwordResponse, passwordReq)
	if passwordResponse.Code != http.StatusOK {
		t.Fatalf("password reset status = %d, body = %s", passwordResponse.Code, passwordResponse.Body.String())
	}
	assertOpenAPIExchange(t, "adminResetUserPassword", passwordReq, passwordResponse)
}

func TestAdminUpdateUser_RejectsDemotingLastAdmin(t *testing.T) {
	app := setupSessionTestApp(t)

	target := createTestUser(t, app, "Solo Admin", "solo-admin@example.com", true)
	handler := authenticatedRouter(t, app, target.ID)

	body := `{"name":"Solo Admin","email":"solo-admin@example.com","is_admin":false}`
	req := newOpenAPIJSONRequest(http.MethodPatch, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), body)

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

	target := createTestUser(t, app, "Admin One", "admin-one@example.com", true)
	actor := createTestUser(t, app, "Admin Two", "admin-two@example.com", true)
	handler := authenticatedRouter(t, app, actor.ID)

	body := `{"name":"Admin One","email":"admin-one@example.com","is_admin":false}`
	req := newOpenAPIJSONRequest(http.MethodPatch, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), body)

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

	target := createTestUser(t, app, "Only Admin", "only-admin@example.com", true)
	handler := authenticatedRouter(t, app, target.ID)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), nil)
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

	target := createTestUser(t, app, "Delete Me", "delete-me@example.com", true)
	actor := createTestUser(t, app, "Keep Me", "keep-me@example.com", true)
	handler := authenticatedRouter(t, app, actor.ID)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), nil)
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
