package main

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidatePasswordCountsRunes(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "eight multibyte characters is too short",
			value:   strings.Repeat("界", 8),
			wantErr: true,
		},
		{
			name:  "nine multibyte characters is accepted",
			value: strings.Repeat("界", 9),
		},
		{
			name:  "one hundred twenty eight multibyte characters is accepted",
			value: strings.Repeat("界", 128),
		},
		{
			name:    "one hundred twenty nine multibyte characters is too long",
			value:   strings.Repeat("界", 129),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.value, "password")
			if tt.wantErr && err == nil {
				t.Fatal("validatePassword returned nil error, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validatePassword returned error: %v", err)
			}
		})
	}
}

func TestUserProfileMutationHandlers_ConformToOpenAPI(t *testing.T) {
	app := setupTestApp(t)
	defer app.DB.Close()

	app.Config.StaticDir = t.TempDir()
	err := app.InitSettings(context.Background())
	if err != nil {
		t.Fatalf("initialize settings: %v", err)
	}
	app.InitSession()
	app.InitRouter()

	user := createTestUserWithPassword(t, app, "Original", "original@example.com", "current password")
	cookie := newAuthSessionCookie(t, app, user.ID)

	tests := []struct {
		name        string
		operationID string
		target      string
		body        string
	}{
		{name: "name", operationID: "updateUserName", target: "/api/user/name", body: `{"name":"Updated Name"}`},
		{name: "email", operationID: "updateUserEmail", target: "/api/user/email", body: `{"email":"updated@example.com"}`},
		{name: "password", operationID: "updateUserPassword", target: "/api/user/password", body: `{"current_password":"current password","new_password":"updated password"}`},
		{name: "avatar", operationID: "updateUserAvatar", target: "/api/user/avatar", body: `{"avatar":"https://example.com/avatar.png"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := newOpenAPIJSONRequest(http.MethodPut, test.target, test.body)
			req.AddCookie(cookie)
			response := httptest.NewRecorder()
			app.Router.ServeHTTP(response, req)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			assertOpenAPIExchange(t, test.operationID, req, response)
		})
	}

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	part, err := writer.CreateFormFile("avatar", "avatar.png")
	if err != nil {
		t.Fatalf("create avatar form file: %v", err)
	}
	_, err = part.Write([]byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'})
	if err != nil {
		t.Fatalf("write avatar form file: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("close multipart body: %v", err)
	}

	uploadReq := newOpenAPIRequest(http.MethodPost, "/api/user/avatar/upload", writer.FormDataContentType(), multipartBody.Bytes())
	uploadReq.AddCookie(cookie)
	uploadResponse := httptest.NewRecorder()
	app.Router.ServeHTTP(uploadResponse, uploadReq)
	if uploadResponse.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	assertOpenAPIExchange(t, "uploadUserAvatar", uploadReq, uploadResponse)
}
