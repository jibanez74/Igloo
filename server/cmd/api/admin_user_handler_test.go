package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
	"igloo/cmd/internal/helpers"

	"github.com/go-chi/chi/v5"
)

func setupAdminUserHTTPTestApp(t *testing.T) *Application {
	t.Helper()

	app := setupTestApp(t)
	app.InitSession()

	return app
}

func createAdminUser(t *testing.T, app *Application, name, email string, isAdmin bool) database.User {
	t.Helper()

	user, err := app.Queries.CreateUser(context.Background(), database.CreateUserParams{
		Name:     name,
		Email:    email,
		Password: "hashed",
		IsAdmin:  isAdmin,
		Avatar:   sql.NullString{},
	})
	if err != nil {
		t.Fatalf("create admin user %q: %v", email, err)
	}

	return user
}

func mountAdminUserRouter(app *Application, userID int64) http.Handler {
	r := chi.NewRouter()
	r.Patch("/api/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.AdminUpdateUser(w, r)
	})
	r.Delete("/api/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		app.SessionManager.Put(r.Context(), helpers.COOKIE_USER_ID, userID)
		app.AdminDeleteUser(w, r)
	})

	return app.SessionManager.LoadAndSave(r)
}

func TestAdminUpdateUser_RejectsDemotingLastAdmin(t *testing.T) {
	app := setupAdminUserHTTPTestApp(t)
	defer app.DB.Close()

	target := createAdminUser(t, app, "Solo Admin", "solo-admin@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	body := `{"name":"Solo Admin","email":"solo-admin@example.com","is_admin":false}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

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
	app := setupAdminUserHTTPTestApp(t)
	defer app.DB.Close()

	target := createAdminUser(t, app, "Admin One", "admin-one@example.com", true)
	createAdminUser(t, app, "Admin Two", "admin-two@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	body := `{"name":"Admin One","email":"admin-one@example.com","is_admin":false}`
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

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
	app := setupAdminUserHTTPTestApp(t)
	defer app.DB.Close()

	target := createAdminUser(t, app, "Only Admin", "only-admin@example.com", true)
	handler := mountAdminUserRouter(app, 999)

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
	app := setupAdminUserHTTPTestApp(t)
	defer app.DB.Close()

	target := createAdminUser(t, app, "Delete Me", "delete-me@example.com", true)
	createAdminUser(t, app, "Keep Me", "keep-me@example.com", true)
	handler := mountAdminUserRouter(app, 999)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/users/"+strconv.FormatInt(target.ID, 10), nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

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
