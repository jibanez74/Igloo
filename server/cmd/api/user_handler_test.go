package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"igloo/cmd/internal/helpers"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserProfileMutationHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupSessionTestApp(t)

	user := createTestUser(t, app, "Original", "original@example.com", false)
	handler := authenticatedRouter(t, app, user.ID)

	tests := []struct {
		name        string
		operationID string
		target      string
		body        string
	}{
		{name: "name", operationID: "updateUserName", target: "/api/user/name", body: `{"name":"Updated Name"}`},
		{name: "email", operationID: "updateUserEmail", target: "/api/user/email", body: `{"email":"updated@example.com"}`},
		{name: "password", operationID: "updateUserPassword", target: "/api/user/password", body: fmt.Sprintf(`{"current_password":%q,"new_password":"updated password"}`, testUserPassword)},
		{name: "avatar", operationID: "updateUserAvatar", target: "/api/user/avatar", body: `{"avatar":"https://example.com/avatar.png"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serveOpenAPIExchange(t, handler, test.operationID, newOpenAPIJSONRequest(http.MethodPut, test.target, test.body), http.StatusOK)
		})
	}

	pngHeader := []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'}
	serveOpenAPIExchange(t, handler, "uploadUserAvatar", avatarUploadRequest(t, "avatar", "avatar.png", pngHeader), http.StatusOK)
}

func TestUserProfileMutations_RejectInvalidInput(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Original", "original@example.com", false)
	createTestUser(t, app, "Taken", "taken@example.com", false)
	handler := authenticatedRouter(t, app, user.ID)

	tests := []struct {
		name        string
		path        string
		body        string
		wantStatus  int
		wantMessage string
	}{
		{"empty name", "/api/user/name", `{"name":""}`, http.StatusBadRequest, nameRequiredMessage},
		{"overlong name", "/api/user/name", fmt.Sprintf(`{"name":%q}`, strings.Repeat("é", userNameMaxLength+1)), http.StatusBadRequest, fmt.Sprintf("name must be %d characters or less", userNameMaxLength)},
		{"empty email", "/api/user/email", `{"email":""}`, http.StatusBadRequest, emailRequiredMessage},
		{"overlong email", "/api/user/email", fmt.Sprintf(`{"email":%q}`, strings.Repeat("a", userEmailMaxLength+1)), http.StatusBadRequest, fmt.Sprintf("email must be %d characters or less", userEmailMaxLength)},
		{"taken email", "/api/user/email", `{"email":"taken@example.com"}`, http.StatusConflict, "that email address is already in use"},
		{"missing passwords", "/api/user/password", `{"current_password":"","new_password":"replacement pw"}`, http.StatusBadRequest, "current and new password are required"},
		{"wrong current password", "/api/user/password", `{"current_password":"not it","new_password":"replacement pw"}`, http.StatusUnauthorized, "current password is incorrect"},
		{"malformed avatar body", "/api/user/avatar", `{"avatar":`, http.StatusBadRequest, invalidRequestBodyMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			var resp helpers.JSONResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !resp.Error || resp.Message != tt.wantMessage {
				t.Fatalf("response = %+v, want error %q", resp, tt.wantMessage)
			}
		})
	}

	stored, err := app.Queries.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Original" || stored.Email != "original@example.com" || stored.Password != user.Password {
		t.Fatalf("rejected requests changed the user: %+v", stored)
	}
}

func avatarUploadRequest(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, err = part.Write(content)
	if err != nil {
		t.Fatalf("write form file: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	// Built through newOpenAPIRequest so the contract check can replay it.
	return newOpenAPIRequest(http.MethodPost, "/api/user/avatar/upload", writer.FormDataContentType(), body.Bytes())
}

func TestUploadUserAvatar_RejectsMissingAndNonImageFiles(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Uploader", "uploader@example.com", false)
	handler := authenticatedRouter(t, app, user.ID)

	tests := []struct {
		name        string
		request     *http.Request
		wantMessage string
	}{
		{"missing avatar field", avatarUploadRequest(t, "file", "avatar.png", []byte("\x89PNG\r\n\x1a\n")), "no file uploaded"},
		{"non-image content", avatarUploadRequest(t, "avatar", "avatar.txt", []byte("plain text is not an image")), "invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, tt.request)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
			}
			var resp helpers.JSONResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			if err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !resp.Error || resp.Message != tt.wantMessage {
				t.Fatalf("response = %+v, want %q", resp, tt.wantMessage)
			}
		})
	}

	stored, err := app.Queries.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Avatar.Valid {
		t.Fatalf("rejected upload stored an avatar: %q", stored.Avatar.String)
	}
}

// Uploaded avatars live under the static directory; replacing or clearing one
// must remove the previous file so the directory does not fill with orphans.
func TestUserAvatar_ReplacingOrClearingRemovesTheUploadedFile(t *testing.T) {
	app := setupSessionTestApp(t)
	user := createTestUser(t, app, "Uploader", "uploader@example.com", false)
	handler := authenticatedRouter(t, app, user.ID)
	avatarsDir := filepath.Join(app.CurrentSettings().StaticDir, "avatars")

	upload := func(t *testing.T, filename string, content []byte) string {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, avatarUploadRequest(t, "avatar", filename, content))
		if w.Code != http.StatusOK {
			t.Fatalf("upload %s status = %d: %s", filename, w.Code, w.Body.String())
		}
		stored, err := app.Queries.GetUser(context.Background(), user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !stored.Avatar.Valid || !strings.HasPrefix(stored.Avatar.String, "/api/static/avatars/") {
			t.Fatalf("stored avatar = %+v, want an uploaded avatar URL", stored.Avatar)
		}
		return filepath.Join(avatarsDir, strings.TrimPrefix(stored.Avatar.String, "/api/static/avatars/"))
	}
	fileExists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}

	pngPath := upload(t, "one.png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"))
	if !fileExists(pngPath) {
		t.Fatalf("uploaded avatar %s is missing", pngPath)
	}

	gifPath := upload(t, "two.gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00"))
	if !fileExists(gifPath) {
		t.Fatalf("replacement avatar %s is missing", gifPath)
	}
	if fileExists(pngPath) {
		t.Fatalf("replaced avatar %s was not deleted", pngPath)
	}

	req := newOpenAPIJSONRequest(http.MethodPut, "/api/user/avatar", `{"avatar":"https://example.com/avatar.png"}`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("set avatar URL status = %d: %s", w.Code, w.Body.String())
	}
	if fileExists(gifPath) {
		t.Fatalf("avatar %s survived switching to an external URL", gifPath)
	}
	stored, err := app.Queries.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Avatar.String != "https://example.com/avatar.png" {
		t.Fatalf("stored avatar = %+v, want the external URL", stored.Avatar)
	}

	cleared := newOpenAPIJSONRequest(http.MethodPut, "/api/user/avatar", `{"avatar":""}`)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, cleared)
	if w.Code != http.StatusOK {
		t.Fatalf("clear avatar status = %d: %s", w.Code, w.Body.String())
	}
	stored, err = app.Queries.GetUser(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Avatar.Valid {
		t.Fatalf("cleared avatar = %+v, want NULL", stored.Avatar)
	}
}

func TestDeleteUserAccount_RefusesAdminsAndEndsTheSession(t *testing.T) {
	app := setupSessionTestApp(t)
	admin := createTestUser(t, app, "Admin", "admin@example.com", true)
	member := createTestUser(t, app, "Member", "member@example.com", false)

	w := httptest.NewRecorder()
	authenticatedRouter(t, app, admin.ID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/user", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin delete status = %d, want 403: %s", w.Code, w.Body.String())
	}
	_, err := app.Queries.GetUser(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("admin row after refused delete: %v", err)
	}

	memberHandler := authenticatedRouter(t, app, member.ID)
	w = httptest.NewRecorder()
	memberHandler.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/user", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("member delete status = %d, want 200: %s", w.Code, w.Body.String())
	}
	_, err = app.Queries.GetUser(context.Background(), member.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("member row after delete: %v, want sql.ErrNoRows", err)
	}

	// The same cookie is now anonymous: the session was destroyed with the account.
	w = httptest.NewRecorder()
	memberHandler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/devices/", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status after account deletion = %d, want 401: %s", w.Code, w.Body.String())
	}
}
