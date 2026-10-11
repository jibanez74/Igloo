package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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

	missingReq := newOpenAPIJSONRequest(http.MethodPut, "/api/admin/users/999999/password", passwordBody)
	serveOpenAPIExchange(t, handler, "adminResetUserPassword", missingReq, http.StatusNotFound)
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

// Required body fields must be sent: a missing is_admin used to read as
// false and demote an administrator, a missing pin removed the PIN, and a
// missing avatar cleared it.
func TestRequiredUserFieldsAreNotDefaulted(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Admin", "required-admin@example.com", true)
	other := createTestUser(t, app, "Other Admin", "required-other@example.com", true)
	adminRouter := authenticatedRouter(t, app, admin.ID)
	longEmail := strings.Repeat("a", 256)

	reject := func(t *testing.T, handler http.Handler, operationID, method, target, body string) {
		t.Helper()
		req := newOpenAPIJSONRequest(method, target, body)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assertOpenAPIResponse(t, operationID, req, w)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s %s %s status = %d, want 400: %s", method, target, body, w.Code, w.Body.String())
		}
	}

	otherPath := "/api/admin/users/" + strconv.FormatInt(other.ID, 10)
	reject(t, adminRouter, "adminUpdateUser", http.MethodPatch, otherPath, `{"name":"Other Admin","email":"required-other@example.com"}`)
	reject(t, adminRouter, "adminUpdateUser", http.MethodPatch, otherPath, `{"name":"Other Admin","email":"required-other@example.com","is_admin":null}`)
	reject(t, adminRouter, "adminUpdateUser", http.MethodPatch, otherPath, fmt.Sprintf(`{"name":"Other Admin","email":%q,"is_admin":true}`, longEmail))
	stored, err := app.Queries.GetUser(context.Background(), other.ID)
	if err != nil || !stored.IsAdmin {
		t.Fatalf("other admin = %+v (%v), want still an administrator", stored, err)
	}

	reject(t, adminRouter, "adminCreateUser", http.MethodPost, "/api/admin/users", `{"name":"New","email":"required-new@example.com","password":"long-enough-password"}`)
	reject(t, adminRouter, "adminCreateUser", http.MethodPost, "/api/admin/users", fmt.Sprintf(`{"name":"New","email":%q,"password":"long-enough-password","is_admin":false}`, longEmail))
	_, err = app.Queries.GetUserByEmail(context.Background(), "required-new@example.com")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected create stored a user: %v", err)
	}

	member := createTestUser(t, app, "Member", "required-member@example.com", false)
	memberRouter := authenticatedRouter(t, app, member.ID)
	serveOpenAPIExchange(t, memberRouter, "updateUserPin", newOpenAPIJSONRequest(http.MethodPut, "/api/user/pin", `{"pin":"1234"}`), http.StatusOK)
	serveOpenAPIExchange(t, memberRouter, "updateUserAvatar", newOpenAPIJSONRequest(http.MethodPut, "/api/user/avatar", `{"avatar":"https://example.com/a.png"}`), http.StatusOK)
	reject(t, memberRouter, "updateUserPin", http.MethodPut, "/api/user/pin", `{"current_pin":"1234"}`)
	reject(t, memberRouter, "updateUserAvatar", http.MethodPut, "/api/user/avatar", `{}`)
	stored, err = app.Queries.GetUser(context.Background(), member.ID)
	if err != nil || stored.Pin.String != "1234" || stored.Avatar.String != "https://example.com/a.png" {
		t.Fatalf("member = %+v (%v), want the PIN and avatar kept", stored, err)
	}
}

// A user row that is gone means a stale session (401); any other failure to
// read the admin flag is the server's (500), not the caller's.
func TestRequireAdmin_SeparatesStaleSessionsFromDatabaseFailures(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Admin", "require-admin@example.com", true)
	gone := createTestUser(t, app, "Gone", "require-gone@example.com", false)
	adminRouter := authenticatedRouter(t, app, admin.ID)
	goneRouter := authenticatedRouter(t, app, gone.ID)

	err := app.Queries.DeleteUser(context.Background(), gone.ID)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w := httptest.NewRecorder()
	goneRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("deleted user status = %d, want 401: %s", w.Code, w.Body.String())
	}

	// The session read is served from the session cache, so only the admin
	// check reaches the closed database.
	err = app.DB.Close()
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	w = httptest.NewRecorder()
	adminRouter.ServeHTTP(w, req)
	assertOpenAPIExchange(t, "adminGetUsers", req, w)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("database failure status = %d, want 500: %s", w.Code, w.Body.String())
	}
}
